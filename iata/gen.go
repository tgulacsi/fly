//go:build never

// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/google/renameio/v2"
	"github.com/kr/pretty"
	"github.com/tgulacsi/fly/airline"
	"github.com/tgulacsi/fly/iata"
)

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

func Main() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	sr, err := airline.NewClient(nil).Get(ctx, "https://davidmegginson.github.io/ourairports-data/airports.csv")
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
	bw := bufio.NewWriter(fh)
	bw.WriteString(`package iata
// GENERATED

func init() {
	Airports = map[string]Airport{
`)
	for {
		row, err := cr.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		a := reflect.ValueOf(&iata.Airport{})
		for _, f := range m {
			if f.Convert == nil {
				a.Elem().Field(f.Field).SetString(row[f.Column])
			} else {
				x, err := f.Convert(row[f.Column])
				if err != nil {
					return fmt.Errorf("convert %q: %w", row[f.Column], err)
				}
				a.Elem().Field(f.Field).Set(reflect.ValueOf(x))
			}
		}
		v := a.Elem().Interface().(iata.Airport)
		if v.IATACode != "" {
			s := strings.Replace(pretty.Sprintf("%# v", v), "iata.", "", 1)
			pretty.Fprintf(bw, "%q: %s,\n", v.IATACode, s)
		}
	}
	bw.WriteString(`
	}
}
`)
	bw.Flush()
	return fh.CloseAtomicallyReplace()
}
