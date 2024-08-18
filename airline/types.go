// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package airline

import (
	"time"
)

type Airport struct {
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

type Fare struct {
	Arrival     time.Time `json:"arrivalDate"`
	Departure   time.Time `json:"departureDate"`
	Airline     string    `json:"airline"`
	Source      string    `json:"source"`
	Origin      string    `json:"origin"`
	Destination string    `json:"destination"`
	Day         string    `json:"day"`
	Currency    string    `json:"currency"`
	Price       float64   `json:"price"`
}
