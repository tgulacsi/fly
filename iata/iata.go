// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package iata

import (
	"context"
	_ "embed"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/alecthomas/mph"
	"github.com/fxamacker/cbor/v2"
)

//go:generate flatc --go --go-namespace fbs iata.fbs
//go:generate go run ./gen.go
//go:generate go fmt

// "id","ident","type","name","latitude_deg","longitude_deg","elevation_ft","continent","iso_country","iso_region","municipality","scheduled_service","gps_code","iata_code","local_code","home_link","wikipedia_link","keywords"
// 4296,"LHBP","large_airport","Budapest Liszt Ferenc International Airport",47.43018,19.262393,495,"EU","HU","HU-BU","Budapest","yes","LHBP","BUD",,"http://www.bud.hu/english","https://en.wikipedia.org/wiki/Budapest_Ferenc_Liszt_International_Airport","Ferihegyi nemzetközi repülőtér, Budapest Liszt Ferenc international Airport"

type lookup struct {
	mu       sync.RWMutex
	m        map[string]Airport
	nameCode map[string]string
	once     sync.Once
}

var airports lookup

type Airport struct {
	Location              *time.Location `csv:"-"`
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
	serialized            []byte
	Lat                   float64 `csv:"latitude_deg"`
	Lon                   float64 `csv:"longitude_deg"`
}

func (l *lookup) init() {
	l.once.Do(func() {
		l.nameCode = make(map[string]string, len(l.m))
		var buf strings.Builder
		for k, v := range l.m {
			if len(v.serialized) != 0 {
				type minimal struct {
					TimeZone, Name, Municipality string
				}
				var aMin minimal
				if err := cbor.Unmarshal(v.serialized, &aMin); err != nil {
					panic(err)
				}
				v.TimeZone, v.Name, v.Municipality = aMin.TimeZone, aMin.Name, aMin.Municipality
			}

			if v.Location == nil {
				v.Location, _ = time.LoadLocation(v.TimeZone)
				l.m[k] = v
			}

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
	k := nameOrCode
	l.mu.RLock()
	a, ok := l.m[k]
	l.mu.RUnlock()
	if !ok {
		for _, nm := range append(
			[]string{
				nameOrCode, strings.TrimSuffix(nameOrCode, "-intl"),
			},
			strings.FieldsFunc(
				nameOrCode,
				func(r rune) bool { return r == '-' || r == ' ' },
			)...) {
			if k, ok = l.nameCode[nm]; ok {
				l.mu.RLock()
				a = l.m[k]
				l.mu.RUnlock()
				break
			}
		}
		if !ok {
			slog.Warn("not found", "nameOrCode", nameOrCode)
			return Airport{}, false
		}
	}
	if len(a.serialized) != 0 {
		if err := cbor.Unmarshal(a.serialized, &a); err != nil {
			panic(err)
		}
		a.serialized = nil
		if a.Location == nil {
			a.Location, _ = time.LoadLocation(a.TimeZone)
		}
		l.mu.Lock()
		l.m[k] = a
		l.mu.Unlock()
	}
	return a, true
}

func (l *lookup) Codes(onlyLarge bool) []string {
	l.init()
	l.mu.RLock()
	defer l.mu.RUnlock()
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

//go:embed codes.dat
var codesDAT []byte

func init() {
	if len(codesDAT) == 0 {
		return
	}
	m, err := mph.Mmap(codesDAT)
	if err != nil {
		panic(err)
	}
	airports.m = make(map[string]Airport, m.Len())
	start := time.Now()
	for it := m.Iterate(); it != nil; it = it.Next() {
		k, v := it.Get()
		airports.m[string(k)] = Airport{serialized: v}
	}
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("deserialization", "dur", time.Since(start), "n", len(airports.m))
		start = time.Now()
		a := airports.Get("BUD")
		slog.Debug("BUD", "a", a, "dur", time.Since(start))
	}
}
