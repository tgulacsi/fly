// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/tgulacsi/fly/airline"
	"github.com/tgulacsi/fly/easyjet"
	"github.com/tgulacsi/fly/gflights"
	"github.com/tgulacsi/fly/iata"
	"github.com/tgulacsi/fly/ryanair"
	"github.com/tgulacsi/fly/wizzair"
)

func main() {
	if err := Main(); err != nil {
		slog.Error("Main", "error", err)
		os.Exit(1)
	}
}
func Main() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx = airline.WithLogger(ctx, slog.Default())

	rar := ryanair.Ryanair{Client: airline.NewClient(nil, false)}
	ej := easyjet.EasyJet{Client: airline.NewClient(nil, true)}
	wz, err := wizzair.New(ctx, nil)
	if err != nil {
		return err
	}
	G, err := gflights.New(ctx)
	if err != nil {
		return err
	}
	airlines := map[string]airline.Airline{
		"ryanair": rar, "easyjet": ej, "wizzair": wz,
		"gflights": G,
	}
	// airlines = airlines[2:3]

	origin := "BUD"
	FS := flag.NewFlagSet("destinations", flag.ContinueOnError)
	FS.StringVar(&origin, "origin", origin, "origin")
	destinationsCmd := ffcli.Command{Name: "destinations", FlagSet: FS,
		Exec: func(ctx context.Context, args []string) error {
			destinations, err := rar.Destinations(ctx, origin)
			for _, d := range destinations {
				fmt.Println(d)
			}
			return err
		},
	}
	currency := "EUR"
	var under float64
	FS = flag.NewFlagSet("fares", flag.ContinueOnError)
	FS.StringVar(&currency, "currency", currency, "currency")
	FS.StringVar(&origin, "origin", origin, "origin")
	FS.Float64Var(&under, "under", 50, "list only under this price")
	faresCmd := ffcli.Command{Name: "fares", FlagSet: FS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("need date, got only %d", len(args))
			}
			departDate, err := time.ParseInLocation("20060102", strings.Map(func(r rune) rune {
				if '0' <= r && r <= '9' {
					return r
				}
				return -1
			}, args[0])[:8], time.Local)
			var destination string
			if len(args) > 1 {
				destination = args[1]
			}
			if err != nil {
				return fmt.Errorf("parse %q as 2006-01-02: %w", args[1], err)
			}
			cmpFare := func(a, b airline.Fare) int {
				if a.Currency != b.Currency {
					slog.Warn("currency mismatch", "a", a, "b", b)
				} else {
					if a.Price < b.Price {
						return -1
					} else if a.Price > b.Price {
						return 1
					}
				}
				if a.Day < b.Day {
					return -1
				} else if a.Day > b.Day {
					return 1
				}
				if a.Origin == b.Origin {
					if a.Destination < b.Destination {
						return -1
					} else if a.Destination > b.Destination {
						return 1
					}
				}
				return 0
			}

			type Stat struct {
				Dur time.Duration
				N   int
			}
			stats := make(map[string]Stat, len(airlines))
			var mu sync.Mutex
			var fares []airline.Fare
			grp, grpCtx := errgroup.WithContext(ctx)
			for name, f := range airlines {
				f := f
				grp.Go(func() error {
					var local []airline.Fare
					start := time.Now()
					if destination == "" {
						local, err = airline.WithAllFares(f).
							AllFares(grpCtx, origin, departDate, currency)
					} else {
						local, err = f.
							Fares(grpCtx, origin, destination, departDate, currency)
					}
					dur := time.Since(start)
					if err != nil {
						err = fmt.Errorf("%s: %w", name, err)
					}
					slices.SortFunc(local, cmpFare)
					for i, f := range local {
						// round to .50
						f.Price = math.Round(f.Price*2.0) / 2.0
						local[i] = f
					}
					mu.Lock()
					fares = append(fares, local...)
					stats[name] = Stat{Dur: dur, N: len(local)}
					mu.Unlock()
					return err
				})
			}
			if err := grp.Wait(); err != nil {
				return err
			}
			slices.SortStableFunc(fares, cmpFare)
			var min float64
			var found bool
			for _, f := range slices.Compact(fares) {
				if f.Currency != currency {
					slog.Warn("currency mismatch", "wanted", currency, "got", f)
				}
				if f.Price > under {
					if min < under || min > f.Price {
						min = f.Price
					}
					continue
				}
				if f.Destination == "" {
					slog.Warn("no destination", "got", f)
				}
				dest := iata.Get(f.Destination)
				fmt.Printf("% 3.2f\t%s\t%s (%s, %s)\t%s[%s]\n",
					f.Price, f.Day,
					f.Destination, dest.Country, dest.Municipality,
					f.Airline, f.Source)
				found = true
			}
			if !found {
				if min > 0 {
					slog.Warn("No flight found", "under", under, "min", min)
				} else {
					slog.Warn("No flight found.")
				}
			}
			slog.Info("statistics", "stats", stats)
			return err
		},
	}
	app := ffcli.Command{Name: "fly", Subcommands: []*ffcli.Command{
		&destinationsCmd, &faresCmd,
	}}
	return app.ParseAndRun(ctx, os.Args[1:])
}
