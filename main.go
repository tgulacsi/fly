package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/tgulacsi/fly/airline"
	"github.com/tgulacsi/fly/ryanair"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func main() {
	if err := Main(); err != nil {
		slog.Error("Main", "error", err)
		os.Exit(1)
	}
}
func Main() error {
	client := airline.NewClient(nil)
	rar := ryanair.Ryanair{Client: client}

	origin := "BUD"
	FS := flag.NewFlagSet("destinations", flag.ContinueOnError)
	FS.StringVar(&origin, "origin", origin, "origin")
	destinationsCmd := ffcli.Command{Name: "destinations", FlagSet: FS,
		Exec: func(ctx context.Context, args []string) error {
			destinations, err := rar.Destinations(ctx, origin)
			for _, d := range destinations {
				fmt.Println(d)
			}
			return err
		},
	}
	currency := "EUR"
	FS = flag.NewFlagSet("fares", flag.ContinueOnError)
	FS.StringVar(&currency, "currency", currency, "currency")
	FS.StringVar(&origin, "origin", origin, "origin")
	faresCmd := ffcli.Command{Name: "fares", FlagSet: FS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("need destination and date, got only %d", len(args))
			}
			destination := args[0]
			departDate, err := time.ParseInLocation("20060102", strings.Map(func(r rune) rune {
				if '0' <= r && r <= '9' {
					return r
				}
				return -1
			}, args[1])[:8], time.Local)
			if err != nil {
				return fmt.Errorf("parse %q as 2006-01-02: %w", args[1], err)
			}
			fares, err := rar.Fares(ctx, origin, destination, departDate, currency)
			for _, f := range fares {
				fmt.Println(f)
			}
			return err
		},
	}
	app := ffcli.Command{Name: "fly", Subcommands: []*ffcli.Command{
		&destinationsCmd, &faresCmd,
	}}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	return app.ParseAndRun(ctx, os.Args[1:])
}
