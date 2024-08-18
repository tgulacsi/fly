//go:build never

// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Idenifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/text/currency"
	"golang.org/x/text/language"

	"github.com/google/renameio/v2"
	"github.com/krisukox/google-flights-api/flights"
	"github.com/tgulacsi/fly/airline"
	"github.com/tgulacsi/fly/iata"
)

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

func Main() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	codes := iata.Codes(true)
	logger := airline.CtxLogger(ctx)
	session, err := flights.New()
	if err != nil {
		return err
	}
	var mu sync.Mutex
	cities := make(map[string]string, 512)
	departure := time.Now().AddDate(0, 0, 1)

	grp, grpCtx := errgroup.WithContext(ctx)
	grp.SetLimit(8)
	for _, c := range codes {
		c := c
		ctx := grpCtx
		grp.Go(func() error {
			var city string
			if ok, err := session.IsIATASupported(ctx, c); err != nil {
				logger.Error("IsIATASupported", "code", c, "error", err)
			} else if ok {
				if city, err = session.AbbrCity(ctx, c, language.English); err != nil {
					if _, found, ok := strings.Cut(err.Error(), " found: "); ok && found != "" {
						city = found
					} else {
						logger.Error("AbbrCity", "code", c, "error", err)
					}
				}
			} else if a, ok := iata.Get(c); ok && a.Municipality != "" {
				if city, err = session.AbbrCity(ctx, a.Municipality, language.English); err != nil {
					if _, found, ok := strings.Cut(err.Error(), " found: "); ok && found != "" {
						city = found
					} else {
						logger.Warn("AbbrCity", "city", a.Municipality, "error", err)
					}
				}
			}
			if city == "" || city[0] == '/' {
				return nil
			}

			// Final check
			_, _, err := session.GetOffers(
				ctx,
				flights.Args{
					Date:       departure,
					ReturnDate: departure.AddDate(0, 0, 37),
					SrcCities:  []string{"London"},
					DstCities:  []string{city},
					Options: flights.Options{
						Travelers: flights.Travelers{Adults: 1},
						Currency:  currency.EUR,
						Stops:     flights.Nonstop,
						Class:     flights.Economy,
						TripType:  flights.OneWay,
						Lang:      language.English,
					},
				},
			)
			if err != nil {
				if errS := err.Error(); !strings.Contains(errS, "could not get the abbreviated") {
					return err
				} else if _, found, ok := strings.Cut(errS, " found: "); ok && found != "" {
					city = found
				}
			}

			mu.Lock()
			cities[c] = city
			mu.Unlock()
			return nil
		})
	}
	if err := grp.Wait(); err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.WriteString(`package gflights
// GENERATED

func init() {
	cities = map[string]string{
`)
	for c, s := range cities {
		fmt.Fprintf(&buf, "\t\t%q: %q,\n", c, s)
	}
	buf.WriteString("}\n}\n")
	return renameio.WriteFile("cities.go", buf.Bytes(), 0644)
}
