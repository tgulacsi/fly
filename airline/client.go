// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package airline

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/cozy/httpcache"
	"github.com/cozy/httpcache/diskcache"
	"github.com/tgulacsi/go/iohlp"
)

type HTTPClient struct {
	client  *http.Client
	prepare func(*http.Request)
}

func NewClient(client *http.Client, cache bool) HTTPClient {
	if client == nil {
		client = http.DefaultClient
	}
	if cache {
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			cacheDir = "/tmp"
		}
		cacheDir = filepath.Join(cacheDir, "airline")
		client.Transport = httpcache.NewTransport(diskcache.New(cacheDir))
	}
	return HTTPClient{client: client}
}
func (c HTTPClient) SetJar(jar *cookiejar.Jar) HTTPClient {
	cl := *c.client
	hct := c.client.Transport.(*httpcache.Transport)
	ht, ok := hct.Transport.(*http.Transport)
	if ok {
		ht = ht.Clone()
	} else {
		ht = http.DefaultTransport.(*http.Transport).Clone()
	}
	cl.Transport = &httpcache.Transport{Transport: ht, Cache: hct.Cache}
	cl.Jar = jar
	return HTTPClient{client: &cl}
}
func (c HTTPClient) SetPrepare(f func(*http.Request)) HTTPClient {
	return HTTPClient{client: c.client, prepare: f}
}

func (c HTTPClient) newRequest(ctx context.Context, method, URL string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, URL, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if prepare, ok := ctx.Value(prepareCtx{}).(prepareRequest); ok {
		prepare(req)
	}
	if c.prepare != nil {
		c.prepare(req)
	}
	return req, nil
}

func (c HTTPClient) Post(ctx context.Context, URL string, body io.Reader) (*io.SectionReader, *http.Response, error) {
	req, err := c.newRequest(ctx, "POST", URL, body)
	if err != nil {
		return nil, nil, err
	}
	return c.do(req)
}
func (c HTTPClient) do(req *http.Request) (*io.SectionReader, *http.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("%s %s: %w: %v", req.Method, req.URL, err, req.Header)
	}
	sr, err := iohlp.MakeSectionReader(resp.Body, 1<<20)
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		var buf strings.Builder
		io.Copy(&buf, sr)
		return nil, resp, fmt.Errorf("%s %s: %s: %s: %s (%s)",
			req.Method, req.URL, resp.Status, buf.String(), req.Header, resp.Header)
	}
	CtxLogger(req.Context()).Debug("request", "body", req.Header, "response", resp.Header)
	return sr, resp, err
}

func (c HTTPClient) Get(ctx context.Context, URL string) (*io.SectionReader, *http.Response, error) {
	req, err := c.newRequest(ctx, "GET", URL, nil)
	if err != nil {
		return nil, nil, err
	}
	return c.do(req)
}

type Airline interface {
	Fares(ctx context.Context, origin string, destination string, departure time.Time, currency string) ([]Fare, error)
	Destinations(ctx context.Context, origin string) ([]string, error)
}
type AirlineAllFares interface {
	Airline
	AllFares(ctx context.Context, origin string, departure time.Time, currency string) ([]Fare, error)
}

func WithAllFares(A Airline) AirlineAllFares {
	if x, ok := A.(AirlineAllFares); ok {
		return x
	}
	return withAllFares{Airline: A}
}

type withAllFares struct{ Airline }

// AllFares returns all the fairs available from the given origin.
func (co withAllFares) AllFares(ctx context.Context, origin string, departure time.Time, currency string) ([]Fare, error) {
	destinations, err := co.Airline.Destinations(ctx, origin)
	if len(destinations) == 0 {
		return nil, err
	}
	grp, grpCtx := errgroup.WithContext(ctx)
	grp.SetLimit(8)
	var mu sync.Mutex
	var fares []Fare
	for _, dest := range destinations {
		dest := dest
		grp.Go(func() error {
			local, err := co.Fares(grpCtx, origin, dest, departure, currency)
			mu.Lock()
			fares = append(fares, local...)
			mu.Unlock()
			return err
		})
	}
	err = grp.Wait()
	return fares, err
}

type prepareCtx struct{}
type prepareRequest func(*http.Request)

func WithPrepare(ctx context.Context, f prepareRequest) context.Context {
	return context.WithValue(ctx, prepareCtx{}, f)
}

type loggerCtx struct{}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerCtx{}, logger)
}
func CtxLogger(ctx context.Context) *slog.Logger {
	if lgr, _ := ctx.Value(loggerCtx{}).(*slog.Logger); lgr != nil {
		return lgr
	}
	return slog.Default()
}
