// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package iata

import (
	"log/slog"
	"strings"
	"sync"
	"unicode"
)

//go:generate go run ./gen.go
//go:generate go fmt

// "id","ident","type","name","latitude_deg","longitude_deg","elevation_ft","continent","iso_country","iso_region","municipality","scheduled_service","gps_code","iata_code","local_code","home_link","wikipedia_link","keywords"
// 4296,"LHBP","large_airport","Budapest Liszt Ferenc International Airport",47.43018,19.262393,495,"EU","HU","HU-BU","Budapest","yes","LHBP","BUD",,"http://www.bud.hu/english","https://en.wikipedia.org/wiki/Budapest_Ferenc_Liszt_International_Airport","Ferihegyi nemzetközi repülőtér, Budapest Liszt Ferenc international Airport"

// TODO[tgulacsi]: try FlatBuffers

type lookup struct {
	m        map[string]Airport
	nameCode map[string]string
	once     sync.Once
}

var airports lookup

type Airport struct {
	ID, Ident, Type, Name string
	Continent             string
	Country               string `csv:"iso_country"`
	Region                string `csv:"iso_region"`
	Municipality          string
	GPSCode               string `csv:"gps_code"`
	IATACode              string `csv:"iata_code"`
	LocalCode             string `csv:"local_code"`
	Home                  string `csv:"home_link"`
	Wikipedia             string `csv:"wikipedia_link"`
	TimeZone              string
	Lat                   float64 `csv:"latitude_deg"`
	Lon                   float64 `csv:"longitude_deg"`
}

func (l *lookup) init() {
	l.once.Do(func() {
		l.nameCode = make(map[string]string, len(l.m))
		var buf strings.Builder
		for k, v := range l.m {
			buf.Reset()
			for _, f := range strings.Fields(strings.ToLower(v.Name)) {
				if f == "international" || f == "air" || strings.Contains(f, "airport") || strings.HasSuffix(f, ".") {
					continue
				}
				l.nameCode[f] = k
				if buf.Len() != 0 {
					buf.WriteByte('-')
				}
				buf.WriteString(f)
			}
			l.nameCode[buf.String()] = k
			s := strings.Map(func(r rune) rune {
				if !unicode.IsLetter(r) {
					return r
				}
				q := r
				for {
					q = unicode.SimpleFold(q)
					if q == r {
						break
					}
					if q <= 255 {
						return q
					}
				}
				return unicode.ToLower(r)
			}, v.Municipality)
			l.nameCode[s] = k
			for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == '/' || r == '-' || r == ' ' || r == ',' }) {
				l.nameCode[f] = k
			}
		}
	})
}
func (l *lookup) Get(nameOrCode string) Airport {
	a, _ := l.Get2(nameOrCode)
	return a
}
func (l *lookup) Get2(nameOrCode string) (Airport, bool) {
	l.init()
	if a, ok := l.m[nameOrCode]; ok {
		return a, ok
	}
	for _, nm := range append([]string{nameOrCode, strings.TrimSuffix(nameOrCode, "-intl")}, strings.FieldsFunc(nameOrCode, func(r rune) bool { return r == '-' || r == ' ' })...) {
		if c, ok := l.nameCode[nm]; ok {
			return l.m[c], true
		}
	}
	slog.Warn("not found", "nameOrCode", nameOrCode)
	return Airport{}, false
}

func (l *lookup) Codes(onlyLarge bool) []string {
	l.init()
	keys := make([]string, 0, len(l.m))
	for k, v := range l.m {
		if onlyLarge && v.Type != "large_airport" {
			continue
		}
		keys = append(keys, k)
	}
	return keys
}

func Get(iataCode string) Airport          { return airports.Get(iataCode) }
func Get2(iataCode string) (Airport, bool) { return airports.Get2(iataCode) }
func Codes(onlyLarge bool) []string        { return airports.Codes(onlyLarge) }
