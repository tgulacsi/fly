// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package gflights

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/text/currency"
	"golang.org/x/text/language"

	"github.com/krisukox/google-flights-api/flights"
	"github.com/tgulacsi/fly/airline"
	"github.com/tgulacsi/fly/iata"
)

//go:generate go run ./gen.go && go fmt
var cities map[string]string

const sourceName = "gflights"

func New(ctx context.Context) (GFlights, error) {
	session, err := flights.New()
	if err != nil {
		return GFlights{}, err
	}
	return GFlights{session: session}, nil
}

type GFlights struct {
	session *flights.Session
}

func (G GFlights) Fares(ctx context.Context, origin, destination string, departure time.Time, curr string) ([]airline.Fare, error) {
	logger := airline.CtxLogger(ctx)
	CURR, err := currency.ParseISO(curr)
	if err != nil {
		return nil, err
	}
	originCity, _ := iata.Get(origin)
	var destCities []string
	if destination != "" {
		destCities = append(destCities, cities[destination])
	} else {
		destCities = make([]string, 0, len(cities))
		for _, s := range cities {
			destCities = append(destCities, s)
		}
	}
	// logger.Info("collected", "cities", destCities)

	var mu sync.Mutex
	var fares []airline.Fare
	grp, grpCtx := errgroup.WithContext(ctx)
	for i := 0; i < 8; i++ {
		remainder := i
		cities := make([]string, 0, len(destCities)/8)
		for i := remainder; i < len(destCities); i += 8 {
			cities = append(cities, destCities[i])
		}
		grp.Go(func() error {
			start := time.Now()
			offers, _, err := G.session.GetOffers(
				grpCtx,
				flights.Args{
					Date:       departure,
					ReturnDate: departure.AddDate(0, 0, 37),
					SrcCities:  []string{originCity.Municipality},
					DstCities:  cities,
					Options: flights.Options{
						Travelers: flights.Travelers{Adults: 1},
						Currency:  CURR,
						Stops:     flights.Nonstop,
						Class:     flights.Economy,
						TripType:  flights.OneWay,
						Lang:      language.English,
					},
				},
			)
			if err != nil {
				return err
			}
			if logger.Enabled(grpCtx, slog.LevelDebug) {
				logger.Debug("gflights", "cities", cities, "dur", time.Since(start).String())
			}

			mu.Lock()
			defer mu.Unlock()
			for _, o := range offers {
				fares = append(fares, airline.Fare{
					Airline:     o.Flight[0].AirlineName,
					Source:      sourceName,
					Day:         o.StartDate.Format("2006-01-02"),
					Arrival:     o.StartDate.Add(o.FlightDuration),
					Departure:   o.StartDate,
					Price:       o.Price,
					Currency:    CURR.String(),
					Origin:      o.SrcAirportCode,
					Destination: o.DstAirportCode,
				})
			}
			return nil
		})
	}
	err = grp.Wait()
	return fares, err
}
