// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package iata

//go:generate go run ./gen.go

// "id","ident","type","name","latitude_deg","longitude_deg","elevation_ft","continent","iso_country","iso_region","municipality","scheduled_service","gps_code","iata_code","local_code","home_link","wikipedia_link","keywords"
// 4296,"LHBP","large_airport","Budapest Liszt Ferenc International Airport",47.43018,19.262393,495,"EU","HU","HU-BU","Budapest","yes","LHBP","BUD",,"http://www.bud.hu/english","https://en.wikipedia.org/wiki/Budapest_Ferenc_Liszt_International_Airport","Ferihegyi nemzetközi repülőtér, Budapest Liszt Ferenc international Airport"

var Airports map[string]Airport

type Airport struct {
	ID, Ident, Type, Name string
	Lat                   float64 `csv:"latitude_deg"`
	Lon                   float64 `csv:"longitude_deg"`
	Continent             string
	Country               string `csv:"iso_counmtry"`
	Region                string `csv:"iso_region"`
	Municipality          string
	GPSCode               string `csv:"gps_code"`
	IATACode              string `csv:"iata_code"`
	LocalCode             string `csv:"local_code"`
	Home                  string `csv:"home_link"`
	Wikipedia             string `csv:"wikipedia_link"`
}
