// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package easyjet

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/tgulacsi/fly/airline"
	"github.com/tgulacsi/fly/iata"
)

type EasyJet struct{ Client airline.HTTPClient }

var _ airline.Airline = EasyJet{}

const baseURL = "https://www.easyjet.com/api/routepricing/v3"
const routesURL = baseURL + "/Routes"

func (ej EasyJet) getRoutes(ctx context.Context) ([]route, error) {
	sr, _, err := ej.Client.Get(
		airline.WithPrepare(ctx, func(r *http.Request) {
			r.Header.Set("Accept", "application/json")
		}),
		routesURL)
	if err != nil {
		return nil, err
	}
	var routes []route
	b, _ := io.ReadAll(sr)
	if err = json.Unmarshal(b, &routes); err == nil && len(routes) == 0 {
		airline.CtxLogger(ctx).Warn("getRoutes", "URL", routesURL, "response", string(b))
	}
	return routes, err
}

func (ej EasyJet) Destinations(ctx context.Context, origin string) ([]string, error) {
	logger := airline.CtxLogger(ctx)
	var destinations []string
	routes, err := ej.getRoutes(ctx)
	if err == nil {
		if len(routes) == 0 {
			return nil, err
		}
		for _, r := range routes {
			if r.Origin == origin {
				destinations = append(destinations, r.Destination)
			}
		}
		return destinations, nil
	}
	logger.Warn("get routes", "error", err)

	sr, _, err := ej.Client.Get(ctx, "https://www.easyjet.com/en/flights-timetables")
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(sr)
	if err != nil {
		return nil, err
	}
	doc.Find("a").Each(func(_ int, sel *goquery.Selection) {
		for _, a := range sel.Nodes[0].Attr {
			if a.Key != "href" {
				continue
			}
			if _, suffix, found := strings.Cut(a.Val, "/cheap-flights/"); found {
				if from, to, found := strings.Cut(suffix, "/"); found {
					logger.Debug("search", "from", from, "origin", origin, "to", to)
					if f, ok := iata.Get2(from); ok && f.IATACode == origin {
						if t, ok := iata.Get2(to); ok {
							destinations = append(destinations, t.IATACode)
						}
					}
				}
			}
		}
	})
	return destinations, nil
}

const (
	sourceName  = "easyjet"
	airlineName = "EasyJet"
)

const searchFaresURL = baseURL + "/searchfares/GetLowestDailyFares?departureAirport={{origin}}&arrivalAirport={{destination}}&currency={{currency}}"

func (ej EasyJet) Fares(ctx context.Context, origin, destination string, departDate time.Time, currency string) ([]airline.Fare, error) {
	logger := airline.CtxLogger(ctx)

	URL := strings.NewReplacer(
		"{{origin}}", origin,
		"{{destination}}", destination,
		"{{currency}}", currency,
	).Replace(searchFaresURL)

	var fares []airline.Fare

	sr, _, err := ej.Client.Get(
		airline.WithPrepare(ctx, func(r *http.Request) {
			r.Header.Set("Accept", "application/json")
		}), URL)
	if err != nil {
		return fares, err
	}
	var local []fare
	b, _ := io.ReadAll(sr)
	if err = json.Unmarshal(b, &local); err == nil && len(local) == 0 {
		logger.Error("got", "body", string(b), "parsed", local)
		return fares, nil
	}

	const timePat = "2006-01-02T15:04:05"
	originTZ := iata.Get(origin).Location

	for _, f := range local {
		departure, _ := time.ParseInLocation(timePat, f.Departure, originTZ)
		arrival, _ := time.ParseInLocation(timePat, f.Arrival, iata.Get(f.Destination).Location)
		fares = append(fares, airline.Fare{
			Source:  sourceName,
			Airline: airlineName,
			Arrival: arrival, Departure: departure,
			Day:    departure.Format("2006-01-02"),
			Origin: f.Origin, Destination: f.Destination,
			Price: f.Price, Currency: currency,
		})
	}

	return fares, err
}

/*
[{"flightNumber":"7173","departureAirport":"BER","arrivalAirport":"BCN","arrivalCountry":"ESP","outboundPrice":172.52,"returnPrice":172.52,"departureDateTime":"2024-08-19T15:10:00","arrivalDateTime":"2024-08-19T17:45:00","serviceError":null},{"flightNumber":"7173","departureAirport":"BER","arrivalAirport":"BCN","arrivalCountry":"ESP","outboundPrice":173.52,"returnPrice":173.52,"departureDateTime":"2024-08-20T15:05:00","arrivalDateTime":"2024-08-20T17:40:00","serviceError":null},
*/
type fare struct {
	FlightNumber string  `json:"flightNumber"`
	Origin       string  `json:"departureAirport"`
	Destination  string  `json:"arrivalAirport"`
	Country      string  `json:"arrivalCountry"`
	Departure    string  `json:"departureDateTime"`
	Arrival      string  `json:"arrivalDateTime"`
	ServiceError string  `json:"serviceError"`
	Price        float64 `json:"outboundPrice"`
	ReturnPrice  float64 `json:"returnPrice"`
}

/*
[

	{
	  "destinationIata": "ATH",
	  "endDate": "2025-06-14T00:00:00",
	  "originIata": "AGP",
	  "startDate": "2024-06-01T00:00:00"
	},
	{
	  "destinationIata": "AGP",
	  "endDate": "2025-06-14T00:00:00",
	  "originIata": "ATH",
	  "startDate": "2024-06-01T00:00:00"
	},
	{
	  "destinationIata": "LYS",
	  "endDate": "2025-06-14T00:00:00",
	  "originIata": "AGP",
	  "startDate": "2023-06-03T00:00:00"
	},
*/
type route struct {
	Destination string `json:"destinationIata"`
	Departure   string `json:"startDate"`
	Arrival     string `json:"endDate"`
	Origin      string `json:"originIata"`
}
