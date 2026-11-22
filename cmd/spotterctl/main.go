// Command spotterctl is the operator CLI for Spotter.
//
// Usage:
//
//	spotterctl ping [-nats-url URL] [-timeout 2s]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/Pohi11/spotter/internal/natsping"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "spotterctl:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] != "ping" {
		return errors.New("usage: spotterctl ping [-nats-url URL] [-timeout 2s]")
	}

	fs := flag.NewFlagSet("ping", flag.ContinueOnError)
	natsURL := fs.String("nats-url", envOr("NATS_URL", nats.DefaultURL), "NATS server URL")
	timeout := fs.Duration("timeout", 2*time.Second, "how long to wait for a reply")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	nc, err := nats.Connect(*natsURL, nats.Name("spotterctl"))
	if err != nil {
		return fmt.Errorf("connect to %s: %w", *natsURL, err)
	}
	defer nc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	res, err := natsping.Ping(ctx, nc, "spotterctl")
	if err != nil {
		return err
	}
	fmt.Printf("reply from %s (%s) in %s\n", res.Reply.GetResponder(), res.Reply.GetRuntime(), res.RoundTrip.Round(time.Microsecond))
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
