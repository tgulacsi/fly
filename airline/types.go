// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package airline

import (
	"time"
)

type Airport struct {
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
	Airline   string    `json:"airline"`
	Source    string    `json:"source"`
	Day       string    `json:"day"`
	Arrival   time.Time `json:"arrivalDate"`
	Departure time.Time `json:"departureDate"`
	Price     float64   `json:"price"`
	Currency  string    `json:"currency"`
}
