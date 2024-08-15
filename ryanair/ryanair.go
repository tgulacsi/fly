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
	"time"

	"github.com/tgulacsi/fly/talk"
)

const airportsURL = `https://www.ryanair.com/api/views/locate/searchWidget/routes/en/airport/{{origin}}`

type Ryanair struct{ Client talk.HTTPClient }

func (co Ryanair) Destinations(ctx context.Context, origin string) ([]ArrivalAirport, error) {
	sr, err := co.Client.Get(ctx, strings.Replace(airportsURL, "{{origin}}", origin, 1))
	if err != nil {
		return nil, err
	}
	var arrivals []struct {
		Airport ArrivalAirport `json:"arrivalAirport"`
	}
	err = json.NewDecoder(sr).Decode(&arrivals)
	arrs := make([]ArrivalAirport, len(arrivals))
	for i, a := range arrivals {
		arrs[i] = a.Airport
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
	Aliases     []string   `json:"aliases"`
	Tags        []string   `json:"tags"`
	Code        string     `json:"code"`
	Name        string     `json:"name"`
	SEO         string     `json:"seoName"`
	Operator    string     `json:"operator"`
	City        NameCode   `json:"city"`
	Region      NameCode   `json:"region"`
	Country     Country    `json:"country"`
	Coordinates Coordinate `json:"coordinates"`
	TimeZone    string     `json:"timeZone"`
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

func (co Ryanair) Fares(ctx context.Context, origin, destination string, departDate time.Time, currency string) ([]Fare, error) {
	sr, err := co.Client.Get(ctx, strings.NewReplacer(
		"{{origin}}", origin,
		"{{destination}}", destination,
		"{{currency}}", currency,
		"{{departDate}}", departDate.Format("2006-01-02"),
	).Replace(faresURL))
	if err != nil {
		return nil, err
	}
	var fares struct {
		Outbound struct {
			Fares []Fare `json:"fares"`
		} `json:"outbound"`
	}
	var buf strings.Builder
	io.Copy(&buf, io.NewSectionReader(sr, 0, sr.Size()))
	slog.Info(buf.String())
	err = json.NewDecoder(sr).Decode(&fares)
	ff := make([]Fare, 0, len(fares.Outbound.Fares))
	for _, f := range fares.Outbound.Fares {
		if f.Unavailable || f.SoldOut || f.Departure == "" {
			continue
		}
		ff = append(ff, f)
	}
	return ff, err
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
	Value               float64 `json:"value"`
	ValueMainUnit       string  `json:"valueMainUnit"`
	ValueFractionalUnit string  `json:"valueFractionalUnit"`
	Currency            string  `json:"currencyCode"`
	Symbol              string  `json:"currencySymbol"`
}
