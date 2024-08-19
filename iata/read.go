// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package iata

import (
	_ "embed"
	"time"
	"log"

	"github.com/remerge/chd"
	"github.com/tgulacsi/fly/iata/fbs"
)

//go:embed codes.dat
var serialized []byte

func init() {
	if len(serialized) == 0 {
		return
	}
	m := chd.NewMap()
	if _, err := m.Read(serialized); err != nil {
		panic(err)
	}
	airports.m = make(map[string]Airport, m.Len())
	start := time.Now()
	m.Values(func(p []byte) bool {
		a := fbs.GetRootAsAirport(p, 0)
		log.Println("a:", a)
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
		return true
	})
	log.Println("deserialization: %s (%d)", time.Since(start), len(airports.m))
}
