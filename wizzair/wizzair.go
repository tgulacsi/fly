// Copyright 2024 Tamás Gulácsi. All rights reserved.
// Copyright @hakkotsu (https://www.postman.com/hakkotsu/ryanair/request/6hzi9pu/get-destinations-from-specific-airport)
//
// SPDX-License-Identifier: Apache-2.0

package wizzair

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/tgulacsi/fly/airline"
	"github.com/tgulacsi/fly/iata"
	// "golang.org/x/net/publicsuffix"
)

const airportsURL = `https://www.ryanair.com/api/views/locate/searchWidget/routes/en/airport/{{origin}}`

func New(client *http.Client) (Wizzair, error) {
	if client == nil {
		client = http.DefaultClient
	}
	jar, err := cookiejar.New(&cookiejar.Options{
		// PublicSuffixList: publicsuffix.List,
	})
	if err != nil {
		return Wizzair{}, err
	}
	client.Jar = jar
	var cookies []*http.Cookie
	wz := Wizzair{client: airline.NewClient(client, false).
		SetPrepare(func(r *http.Request) {
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set(
				"User-Agent", "Mozilla/5.0 (Windows NT 10.0; rv:129.0) Gecko/20100101 Firefox/129.0",
			)
			for _, c := range cookies {
				r.AddCookie(c)
			}
		}),
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, resp, err := wz.client.Get(ctx, "https://wizzair.com/en-gb")
	cancel()
	if err != nil {
		return wz, err
	}
	if err == nil && len(resp.Cookies()) == 0 {
		err = fmt.Errorf("got no cookies from wizzair.com")
	}
	return wz, err
}

type Wizzair struct{ client airline.HTTPClient }

var _ airline.Airline = Wizzair{}

const (
	airlineName = "Wizz Air"
	sourceName  = "wizzair"
)

func (co Wizzair) Destinations(ctx context.Context, origin string) ([]airline.Airport, error) {
	sr, _, err := co.client.Get(ctx, strings.Replace(airportsURL, "{{origin}}", origin, 1))
	if err != nil {
		return nil, err
	}
	var arrivals []struct {
		Airport ArrivalAirport `json:"arrivalAirport"`
	}
	err = json.NewDecoder(sr).Decode(&arrivals)
	arrs := make([]airline.Airport, len(arrivals))
	for i, a := range arrivals {
		A := a.Airport
		arrs[i] = airline.Airport{
			Aliases:  A.Aliases,
			Tags:     A.Tags,
			Code:     A.Code,
			Name:     A.Name,
			SEO:      A.SEO,
			Operator: A.Operator,
			City:     airline.NameCode{Name: A.City.Name, Code: A.City.Code},
			Region:   airline.NameCode(A.Region),
			Country: airline.Country{
				NameCode:       airline.NameCode(A.Country.NameCode),
				Currency:       A.Country.Currency,
				DefaultAirport: A.Country.DefaultAirport,
			},
			Coordinates: airline.Coordinate(A.Coordinates),
			TimeZone:    A.TimeZone,
		}
	}
	return arrs, err
}

/*
[

		{
	    "arrivalAirport": {
	      "aliases": [],
	      "base": true,
	      "city": {
	        "code": "MALAGA",
	        "name": "Malaga"
	      },
	      "code": "AGP",
	      "coordinates": {
	        "latitude": 36.6749,
	        "longitude": -4.49911
	      },
	      "country": {
	        "code": "es",
	        "currency": "EUR",
	        "defaultAirportCode": "BCN",
	        "iso3code": "ESP",
	        "name": "Spain",
	        "schengen": true
	      },
	      "name": "Malaga",
	      "region": {
	        "code": "COSTA_DE_SOL",
	        "name": "Costa del Sol"
	      },
	      "seoName": "malaga",
	      "timeZone": "Europe/Madrid"
	    },
	    "operator": "FR",
	    "recent": false,
	    "seasonal": false,
	    "tags": []
	  },

	  ]
*/
type ArrivalAirport struct {
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
	Base        bool       `json:"base"`
	Recent      bool       `json:"recent"`
	Seasonal    bool       `json:"seasonal"`
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

const faresURL = `https://be.wizzair.com/24.6.0/Api/search/CheapFlights`

type faresReq struct {
	Origin         string `json:"departureStation"`
	Months         int    `json:"months"`
	DiscountedOnly bool   `json:"discountedOnly"`
}

func (co Wizzair) Fares(ctx context.Context, origin, destination string, departDate time.Time, currency string) ([]airline.Fare, error) {
	a, _ := iata.Get(origin)
	originTZ, _ := time.LoadLocation(a.TimeZone)
	months := 1
	now := time.Now()
	for now.AddDate(0, months, 0).Before(departDate) {
		months++
	}
	b, err := json.Marshal(faresReq{Origin: origin, Months: months})
	if err != nil {
		return nil, fmt.Errorf("marshal fares request: %w", err)
	}
	sr, _, err := co.client.Post(ctx, faresURL, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("%s [%s]: %w", faresURL, string(b), err)
	}
	var fares struct {
		Fares []Fare `json:"items"`
	}
	var buf strings.Builder
	io.Copy(&buf, io.NewSectionReader(sr, 0, sr.Size()))
	slog.Info(buf.String())
	err = json.NewDecoder(sr).Decode(&fares)
	ff := make([]airline.Fare, 0, len(fares.Fares))
	for _, f := range fares.Fares {
		const timePat = "2006-01-02T15:04:05"
		departure, err := time.ParseInLocation(timePat, f.Departure, originTZ)
		if err != nil {
			return ff, err
		}
		ff = append(ff, airline.Fare{
			Airline:   airlineName,
			Source:    sourceName,
			Price:     f.RegularPrice.Value,
			Currency:  f.RegularPrice.Currency,
			Departure: departure,
		})
	}
	return ff, err
}

type Fare struct {
	PastPrice            Price  `json:"pastPrice"`
	RegularOriginalPrice Price  `json:"regularOriginalPrice"`
	RegularPrice         Price  `json:"regularPrice"`
	WDCOriginalPrice     Price  `json:"wdcOriginalPrice"`
	WDCPastPrice         Price  `json:"wdcPastPrice"`
	WDCPrice             Price  `json:"wdcPrice"`
	Destination          string `json:"arrivalStation"`
	Currency             string `json:"currencyCode"`
	Origin               string `json:"departureStation"`
	Months               int    `json:"months"`
	DiscountedOnly       bool   `json:"discountedOnly"`
	Departure            string `json:"std"`
}
type Price struct {
	Value    float64 `json:"amount"`
	Currency string  `json:"currencyCode"`
}
