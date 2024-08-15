// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package easyjet

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/tgulacsi/fly/airline"
)

type EasyJet struct{ Client airline.HTTPClient }

var _ airline.Airline = EasyJet{}

func (ej EasyJet) Destinations(ctx context.Context, origin string) ([]airline.Destination, error) {
	sr, err := ej.Client.Get(ctx, "https://www.easyjet.com/en/flights-timetables")
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(sr)
	if err != nil {
		return nil, err
	}
	var destinations []airline.Airport
	doc.Find("a").Each(func(_ int, sel *goquery.Selection) {
		for _, a := range sel.Nodes[0].Attr {
			if a.Key != "href" {
				continue
			}
			if _, suffix, found := strings.Cut(a.Val, "/cheap-flights/"); found {
				if from, to, found := strings.Cut(suffix, "/"); found {
					if from == origin {
						destinations = append(destinations, airline.Airport{Name: to})
					}
				}
			}
		}
	})
	return destinations, nil
}
func (ej EasyJet) Fares(ctx context.Context, origin, destination string, departDate time.Time, currency string) ([]airline.Fare, error) {
	return nil, nil
}

/*
type Request struct {
	AdditionalSeats, AdultSeats int
	ChildSeats int
	Destination string `json:"ArrivalIata"`
	Origin string `json:"DepartureIata"`
	IncludeAdminFees bool
	IncludeFlexiFares bool
	IncludeLowestFareSeats bool
	IncludePrices bool
	IsTransfer bool
	LanguageCode string
	MaxDepartureDate string
	MaxReturnDate string
	MinDepartureDate string
	MinReturnDate string
}
*/
