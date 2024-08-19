// Copyright 2024 Tamás Gulácsi. All rights reserved.
// Copyright @hakkotsu (https://www.postman.com/hakkotsu/ryanair/request/6hzi9pu/get-destinations-from-specific-airport)
//
// SPDX-License-Identifier: Apache-2.0

package ryanair

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/tgulacsi/fly/airline"
	"github.com/tgulacsi/fly/iata"
)

const airportsURL = `https://www.ryanair.com/api/views/locate/searchWidget/routes/en/airport/{{origin}}`

type Ryanair struct{ Client airline.HTTPClient }

var _ airline.Airline = Ryanair{}

const (
	airlineName = "Ryanair"
	sourceName  = "ryanair"
)

func (co Ryanair) Destinations(ctx context.Context, origin string) ([]airline.Airport, error) {
	sr, _, err := co.Client.Get(ctx, strings.Replace(airportsURL, "{{origin}}", origin, 1))
	if err != nil {
		return nil, err
	}
	var arrivals []struct {
		Airport ArrivalAirport `json:"arrivalAirport"`
	}
	err = json.NewDecoder(sr).Decode(&arrivals)
	arrs := make([]airline.Airport, len(arrivals))
	for i, a := range arrivals {
		A := a.Airport
		arrs[i] = airline.Airport{
			Aliases:  A.Aliases,
			Tags:     A.Tags,
			Code:     A.Code,
			Name:     A.Name,
			SEO:      A.SEO,
			Operator: A.Operator,
			City:     airline.NameCode{Name: A.City.Name, Code: A.City.Code},
			Region:   airline.NameCode(A.Region),
			Country: airline.Country{
				NameCode:       airline.NameCode(A.Country.NameCode),
				Currency:       A.Country.Currency,
				DefaultAirport: A.Country.DefaultAirport,
			},
			Coordinates: airline.Coordinate(A.Coordinates),
			TimeZone:    A.TimeZone,
		}
	}
	return arrs, err
}

/*
[

		{
	    "arrivalAirport": {
	      "aliases": [],
	      "base": true,
	      "city": {
	        "code": "MALAGA",
	        "name": "Malaga"
	      },
	      "code": "AGP",
	      "coordinates": {
	        "latitude": 36.6749,
	        "longitude": -4.49911
	      },
	      "country": {
	        "code": "es",
	        "currency": "EUR",
	        "defaultAirportCode": "BCN",
	        "iso3code": "ESP",
	        "name": "Spain",
	        "schengen": true
	      },
	      "name": "Malaga",
	      "region": {
	        "code": "COSTA_DE_SOL",
	        "name": "Costa del Sol"
	      },
	      "seoName": "malaga",
	      "timeZone": "Europe/Madrid"
	    },
	    "operator": "FR",
	    "recent": false,
	    "seasonal": false,
	    "tags": []
	  },

	  ]
*/
type ArrivalAirport struct {
	Country     Country    `json:"country"`
	City        NameCode   `json:"city"`
	Region      NameCode   `json:"region"`
	Code        string     `json:"code"`
	Name        string     `json:"name"`
	SEO         string     `json:"seoName"`
	Operator    string     `json:"operator"`
	TimeZone    string     `json:"timeZone"`
	Aliases     []string   `json:"aliases"`
	Tags        []string   `json:"tags"`
	Coordinates Coordinate `json:"coordinates"`
	Base        bool       `json:"base"`
	Recent      bool       `json:"recent"`
	Seasonal    bool       `json:"seasonal"`
}
type Country struct {
	NameCode
	Currency       string `json:"currency"`
	DefaultAirport string `json:"defaultAirportCode"`
}
type NameCode struct {
	Name string `json:"name"`
	Code string `json:"code"`
}
type Coordinate struct {
	Lat float64 `json:"latitude"`
	Lon float64 `json:"longitude"`
}

const faresURL = `https://www.ryanair.com/api/farfnd/v4/oneWayFares/{{origin}}/{{destination}}/cheapestPerDay?outboundMonthOfDate={{departDate}}&currency={{currency}}`

func (co Ryanair) Fares(ctx context.Context, origin, destination string, departDate time.Time, currency string) ([]airline.Fare, error) {
	logger := airline.CtxLogger(ctx)
	var airports []airline.Airport
	getAirports := func() error {
		var err error
		if len(airports) == 0 {
			airports, err = co.Destinations(ctx, origin)
		}
		return err
	}

	originTZ := iata.Get(origin).Location
	if originTZ == nil {
		getAirports()
		for _, a := range airports {
			if originTZ != nil {
				continue
			}
			backs, _ := co.Destinations(ctx, a.Code)
			for _, a := range backs {
				if a.Code == origin {
					var err error
					if originTZ, err = time.LoadLocation(a.TimeZone); err != nil {
						return nil, err
					}
					break
				}
			}
		}
	}

	var destinations []string
	if destination != "" {
		destinations = []string{destination}
		if _, ok := iata.Get2(destination); !ok {
			if err := getAirports(); err != nil {
				return nil, err
			}
		}
	} else {
		if err := getAirports(); err != nil {
			return nil, err
		}
		destinations = make([]string, len(airports))
		for i, a := range airports {
			destinations[i] = a.Code
		}
	}

	var mu sync.Mutex
	var ff []airline.Fare
	grp, grpCtx := errgroup.WithContext(ctx)
	grp.SetLimit(8)
	for _, dest := range destinations {
		dest := dest
		grp.Go(func() error {
			sr, _, err := co.Client.Get(grpCtx, strings.NewReplacer(
				"{{origin}}", origin,
				"{{destination}}", dest,
				"{{currency}}", currency,
				"{{departDate}}", departDate.Format("2006-01-02"),
			).Replace(faresURL))
			if err != nil {
				return err
			}
			var fares struct {
				Outbound struct {
					Fares []Fare `json:"fares"`
				} `json:"outbound"`
			}
			var buf strings.Builder
			io.Copy(&buf, io.NewSectionReader(sr, 0, sr.Size()))
			if logger.Enabled(ctx, slog.LevelDebug) {
				logger.Debug(buf.String())
			}
			err = json.NewDecoder(sr).Decode(&fares)
			for _, f := range fares.Outbound.Fares {
				if f.Unavailable || f.SoldOut || f.Departure == "" {
					continue
				}
				const timePat = "2006-01-02T15:04:05"
				// slog.Info("fares", "dest", dest, "a", iata.Get(dest))
				arrival, err := time.ParseInLocation(timePat, f.Arrival, iata.Get(dest).Location)
				if err != nil {
					return err
				}
				departure, err := time.ParseInLocation(timePat, f.Departure, originTZ)
				if err != nil {
					return err
				}
				mu.Lock()
				ff = append(ff, airline.Fare{
					Airline:     airlineName,
					Source:      sourceName,
					Origin:      origin,
					Destination: dest,
					Day:         f.Day,
					Price:       f.Price.Value,
					Currency:    f.Price.Currency,
					Arrival:     arrival,
					Departure:   departure,
				})
				mu.Unlock()
			}
			return nil
		})
	}
	return ff, nil
}

type Fare struct {
	Day         string `json:"day"`
	Arrival     string `json:"arrivalDate"`
	Departure   string `json:"departureDate"`
	Price       Price  `json:"price"`
	SoldOut     bool   `json:"soldOut"`
	Unavailable bool   `json:"unavailable"`
}
type Price struct {
	ValueMainUnit       string  `json:"valueMainUnit"`
	ValueFractionalUnit string  `json:"valueFractionalUnit"`
	Currency            string  `json:"currencyCode"`
	Symbol              string  `json:"currencySymbol"`
	Value               float64 `json:"value"`
}
