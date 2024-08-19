// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package iata

import (
	_ "embed"
	// "log"
	"time"

	"github.com/google/flatbuffers/go"
	"github.com/tgulacsi/fly/iata/fbs"
)

//go:embed codes.dat
var serialized []byte

func init() {
	if len(serialized) == 0 {
		return
	}
	airports.m = make(map[string]Airport, 10000)
	// start := time.Now()
	for rest := serialized; len(rest) > 0; {
		size := flatbuffers.GetSizePrefix(rest, 0)
		a := fbs.GetSizePrefixedRootAsAirport(rest[:4+size], 0)
		rest = rest[4+size:]

		// log.Println("a:", a)
		tz := string(a.TimeZone())
		loc, _ := time.LoadLocation(tz)
		airports.m[string(a.IataCode())] = Airport{
			ID:           string(a.IataCode()),
			Ident:        string(a.Ident()),
			Type:         a.Type().String(),
			Name:         string(a.Name()),
			Continent:    string(a.Continent()),
			Country:      string(a.Country()),
			Region:       string(a.Region()),
			Municipality: string(a.Municipality()),
			GPSCode:      string(a.GpsCode()),
			IATACode:     string(a.IataCode()),
			LocalCode:    string(a.LocalCode()),
			Home:         string(a.Home()),
			Wikipedia:    string(a.Wikipedia()),
			TimeZone:     tz,
			Location:     loc,
			Lat:          a.Lat(),
			Lon:          a.Lon(),
		}
	}
	// log.Printf("deserialization: %s (%d)", time.Since(start), len(airports.m))
}
