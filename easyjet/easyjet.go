// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package easyjet

import (
	"context"
	"log/slog"
	"net/url"
	"time"

	"github.com/tgulacsi/fly/airline"
)

type EasyJet struct{ Client airline.HTTPClient }

var _ airline.Airline = EasyJet{}

func (ej EasyJet) Fares(ctx context.Context, origin, destination string, departDate time.Time, currency string) ([]airline.Fare, error) {
	qry := url.Values(map[string][]string{
		"AdultSeats":       {"1"},
		"Destination":      {destination},
		"Origin":           {origin},
		"IncludeAdminFees": {"true"}, "IncludeLowestFareSeats": {"true"},
		"IncludePrices": {"true"}, "LanguageCode": {"EN"},
		"MaxDepartureDate": {departDate.Add(24 * time.Hour).Format("2006-01-02")},
	})
	URL := "https://www.easyjet.com/ejavailability/api/v9/availability/query?" + qry.Encode()
	slog.Info("Fares", "request", URL)
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
