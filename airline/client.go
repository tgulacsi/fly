// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package airline

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/cozy/httpcache"
	"github.com/cozy/httpcache/diskcache"
	"github.com/tgulacsi/go/iohlp"
)

type HTTPClient struct{ client *http.Client }

func NewClient(client *http.Client) HTTPClient {
	if client == nil {
		client = http.DefaultClient
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = "/tmp"
	}
	cacheDir = filepath.Join(cacheDir, "airline")
	cache := diskcache.New(cacheDir)
	client.Transport = httpcache.NewTransport(cache)
	return HTTPClient{client: client}
}

func (c HTTPClient) Get(ctx context.Context, URL string) (*io.SectionReader, error) {
	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	sr, err := iohlp.MakeSectionReader(resp.Body, 1<<20)
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		var buf strings.Builder
		io.Copy(&buf, sr)
		return nil, fmt.Errorf("%s: %s", resp.Status, buf.String())
	}
	return sr, err
}
