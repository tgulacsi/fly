//go:build never

// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/google/flatbuffers/go"
	"github.com/google/renameio/v2"
	"github.com/remerge/chd"

	"github.com/tgulacsi/fly/airline"
	"github.com/tgulacsi/fly/iata"
	"github.com/tgulacsi/fly/iata/fbs"
)

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

func Main() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	client := airline.NewClient(nil, true)
	shortCtx, shortCancel := context.WithTimeout(ctx, time.Minute)
	sr, _, err := client.Get(shortCtx, "https://davidmegginson.github.io/ourairports-data/airports.csv")
	shortCancel()
	if err != nil {
		return err
	}
	cr := csv.NewReader(sr)
	header, err := cr.Read()
	if err != nil {
		return err
	}
	// "id","ident","type","name","latitude_deg","longitude_deg","elevation_ft","continent","iso_country","iso_region","municipality","scheduled_service","gps_code","iata_code","local_code","home_link","wikipedia_link","keywords"
	// 4296,"LHBP","large_airport","Budapest Liszt Ferenc International Airport",47.43018,19.262393,495,"EU","HU","HU-BU","Budapest","yes","LHBP","BUD",,"http://www.bud.hu/english","https://en.wikipedia.org/wiki/Budapest_Ferenc_Liszt_International_Airport","Ferihegyi nemzetközi repülőtér, Budapest Liszt Ferenc international Airport"
	typ := reflect.TypeOf(iata.Airport{})
	type mapping struct {
		Field   int
		Column  int
		Convert func(string) (any, error)
	}
	m := make(map[string]mapping, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		nm := f.Tag.Get("csv")
		if nm == "-" {
			continue
		}
		if nm == "" {
			nm = f.Name
		}
		mp := mapping{Field: i}
		if f.Type.Kind() == reflect.Float64 {
			mp.Convert = func(s string) (any, error) { return strconv.ParseFloat(s, 64) }
		}
		m[strings.ToLower(nm)] = mp
	}
	for i, s := range header {
		k := strings.ToLower(s)
		if f, ok := m[k]; ok {
			f.Column = i
			m[k] = f
		}
	}
	log.Println("mapping:", m)
	fh, err := renameio.NewPendingFile("codes.go")
	if err != nil {
		return err
	}
	defer fh.Cleanup()

	fb := flatbuffers.NewBuilder(0)
	cb := chd.NewBuilder(nil)

	bw := bufio.NewWriter(fh)
	bw.WriteString(`package iata
// GENERATED

func init() {
	airports = lookup{m: map[string]Airport{
`)
	constants := make(map[string]string)
	makeConst := func(s string) string {
		if k, ok := constants[s]; ok {
			return k
		}
		k := fmt.Sprintf("c%04d", len(constants))
		constants[s] = k
		return k
	}
	printTimer := time.NewTicker(10 * time.Second)
	for {
		row, err := cr.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		var v iata.Airport
		rv := reflect.ValueOf(&v)
		for _, f := range m {
			if f.Convert == nil {
				rv.Elem().Field(f.Field).SetString(row[f.Column])
			} else {
				x, err := f.Convert(row[f.Column])
				if err != nil {
					return fmt.Errorf("convert %q: %w", row[f.Column], err)
				}
				rv.Elem().Field(f.Field).Set(reflect.ValueOf(x))
			}
		}
		if v.IATACode == "" {
			continue
		}
		URL := fmt.Sprintf("https://www.timeapi.io/api/timezone/coordinate?latitude=%v&longitude=%v", v.Lat, v.Lon)
		select {
		case <-printTimer.C:
			log.Printf("%.03f%% at %s (%s)", float32(100*cr.InputOffset())/float32(sr.Size()), v.IATACode, URL)
		default:
		}
		shortCtx, shortCancel := context.WithTimeout(ctx, 3*time.Second)
		sr, _, err := client.Get(shortCtx, URL)
		shortCancel()
		if err != nil {
			return err
		}

		type timeZoneResponse struct {
			TimeZone string `json:"timeZone"`
		}
		var tz timeZoneResponse
		if err := json.NewDecoder(sr).Decode(&tz); err != nil {
			return err
		}
		v.TimeZone = tz.TimeZone
		fmt.Fprintf(bw, `%q: {
			ID: %q, Ident: %q, Type: %s,
			Continent: %s, Country: %s, Region: %s, 
			Municipality: %q,
			GPSCode: %q, IATACode: %q, LocalCode: %q, 
			Home: %q, Wikipedia: %q, 
			TimeZone: %s,
			Lat: %v, Lon: %v,
		},
		`,
			v.IATACode,
			v.ID, v.Ident, makeConst(v.Type),
			makeConst(v.Continent), makeConst(v.Country), makeConst(v.Region),
			v.Municipality,
			v.GPSCode, v.IATACode, v.LocalCode,
			v.Home, v.Wikipedia,
			makeConst(v.TimeZone),
			v.Lat, v.Lon,
		)

		fb.Reset()
		sID := fb.CreateString(v.ID)
		sIdent := fb.CreateString(v.Ident)
		sName := fb.CreateString(v.Name)
		sContinent := fb.CreateString(v.Continent)
		sCountry := fb.CreateString(v.Country)
		sRegion := fb.CreateString(v.Region)
		sMunicipality := fb.CreateString(v.Municipality)
		sGPSCode := fb.CreateString(v.GPSCode)
		sIATACode := fb.CreateString(v.IATACode)
		sLocalCode := fb.CreateString(v.LocalCode)
		sHome := fb.CreateString(v.Home)
		sWikipedia := fb.CreateString(v.Wikipedia)
		sTimeZone := fb.CreateString(v.TimeZone)
		fbs.AirportStart(fb)
		fbs.AirportAddId(fb, sID)
		fbs.AirportAddIdent(fb, sIdent)
		fbs.AirportAddType(fb, fbs.EnumValuesType[v.Type])
		fbs.AirportAddName(fb, sName)
		fbs.AirportAddContinent(fb, sContinent)
		fbs.AirportAddCountry(fb, sCountry)
		fbs.AirportAddRegion(fb, sRegion)
		fbs.AirportAddMunicipality(fb, sMunicipality)
		fbs.AirportAddGpsCode(fb, sGPSCode)
		fbs.AirportAddIataCode(fb, sIATACode)
		fbs.AirportAddLocalCode(fb, sLocalCode)
		fbs.AirportAddHome(fb, sHome)
		fbs.AirportAddWikipedia(fb, sWikipedia)
		fbs.AirportAddTimeZone(fb, sTimeZone)
		fbs.AirportAddLat(fb, v.Lat)
		fbs.AirportAddLon(fb, v.Lon)
		fb.Finish(fbs.AirportEnd(fb))

		cb.Add([]byte(v.IATACode), fb.FinishedBytes())
	}
	bw.WriteString(`
	}}
}

const (
`)
	for s, k := range constants {
		fmt.Fprintf(bw, "\t%s = %q\n", k, s)
	}
	bw.WriteString(`
)
`)

	// Build the map
	cm, err := cb.Build()
	if err != nil {
		return err
	}

	// Serialize the map
	{
		fh, err := renameio.NewPendingFile("codes.dat")
		if err != nil {
			return err
		}
		defer fh.Cleanup()
		if n, err := cm.WriteTo(fh); err != nil {
			return err
		} else {
			log.Printf("dat: %d", n)
		}
		fh.CloseAtomicallyReplace()
	}

	// Afterwards, you can deserialize it
	// r, _ := os.Open("mymap.dat")
	// nm := chd.NewMap()
	// nm.Read(r)

	bw.Flush()
	return fh.CloseAtomicallyReplace()
}
