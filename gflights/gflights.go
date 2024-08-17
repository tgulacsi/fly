package gflights

import (
	"context"
	"fmt"
	"time"

	"github.com/krisukox/google-flights-api/flights"
	"github.com/tgulacsi/fly/airline"
	"github.com/tgulacsi/fly/iata"
	"golang.org/x/text/currency"
	"golang.org/x/text/language"
)

const sourceName = "gflights"

func New() (GFlights, error) {
	session, err := flights.New()
	if err != nil {
		return GFlights{}, err
	}
	return GFlights{session: session}, nil
}

type GFlights struct {
	session *flights.Session
}

func (G GFlights) Fares(ctx context.Context, origin, destination string, departure time.Time, curr string) ([]airline.Fare, error) {
	CURR, err := currency.ParseISO(curr)
	if err != nil {
		return nil, err
	}
	originCity, _ := iata.Get(origin)
	destCity, _ := iata.Get(destination)
	offers, priceRange, err := G.session.GetOffers(
		ctx,
		flights.Args{
			Date:       departure,
			ReturnDate: departure.AddDate(0, 0, 37),
			SrcCities:  []string{originCity.Municipality},
			DstCities:  []string{destCity.Municipality},
			Options: flights.Options{
				Travelers: flights.Travelers{Adults: 1},
				Currency:  CURR,
				Stops:     flights.Nonstop,
				Class:     flights.Economy,
				TripType:  flights.OneWay,
				Lang:      language.English,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	if priceRange != nil {
		fmt.Printf("High price %d\n", int(priceRange.High))
		fmt.Printf("Low price %d\n", int(priceRange.Low))
	}
	fares := make([]airline.Fare, 0, len(offers))
	for _, o := range offers {
		fares = append(fares, airline.Fare{
			Airline:   o.Flight[0].AirlineName,
			Source:    sourceName,
			Day:       o.StartDate.Format("2006-01-02"),
			Arrival:   o.StartDate.Add(o.FlightDuration),
			Departure: o.StartDate,
			Price:     o.Price,
		})
	}
	return fares, nil
}
