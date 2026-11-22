package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/Pohi11/spotter/internal/natsping"
)

// wait blocks until every host:port accepts a TCP connection. Init containers use it so a
// service starts only once its dependencies listen (the Go image has no shell).
func wait(args []string) error {
	fs := flag.NewFlagSet("wait", flag.ContinueOnError)
	timeout := fs.Duration("timeout", 5*time.Minute, "give up after this long")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return errors.New("usage: spotterctl wait [-timeout 5m] HOST:PORT...")
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	return waitTCP(ctx, fs.Args(), func(addr string) { fmt.Println("reachable:", addr) })
}

func waitTCP(ctx context.Context, addrs []string, reached func(string)) error {
	var d net.Dialer
	for _, addr := range addrs {
		for {
			conn, err := d.DialContext(ctx, "tcp", addr)
			if err == nil {
				conn.Close()
				reached(addr)
				break
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("%s not reachable: %w", addr, err)
			case <-time.After(time.Second):
			}
		}
	}
	return nil
}

// smoke checks an installed platform end to end; `helm test` runs it (design D6 of change
// phase-09-kubernetes).
func smoke(args []string) error {
	fs := flag.NewFlagSet("smoke", flag.ContinueOnError)
	gatewayURL := fs.String("gateway-url", "http://localhost:8080", "gateway HTTP base URL")
	registryURL := fs.String("registry-url", "http://localhost:8081", "registry HTTP base URL")
	natsURL := fs.String("nats-url", envOr("NATS_URL", nats.DefaultURL), "NATS server URL")
	timeout := fs.Duration("timeout", 2*time.Minute, "how long to keep retrying each check")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	return runSmoke(ctx, smokeConfig{GatewayURL: *gatewayURL, RegistryURL: *registryURL, NATSURL: *natsURL}, func(step string) {
		fmt.Println("ok:", step)
	})
}

type smokeConfig struct{ GatewayURL, RegistryURL, NATSURL string }

// runSmoke runs each check, retrying until it passes or ctx ends, and reports each pass.
func runSmoke(ctx context.Context, cfg smokeConfig, passed func(string)) error {
	client := &http.Client{Timeout: 5 * time.Second}
	name := "smoke-" + uuid.NewString()[:8]
	checks := []struct {
		name string
		fn   func() error
	}{
		{"gateway ready", func() error { return expect(client, "GET", cfg.GatewayURL+"/readyz", nil, http.StatusOK, nil) }},
		{"registry ready", func() error { return expect(client, "GET", cfg.RegistryURL+"/readyz", nil, http.StatusOK, nil) }},
		{"register model " + name + " through the gateway", func() error {
			err := expect(client, "POST", cfg.GatewayURL+"/v1/models", map[string]string{"name": name, "description": "smoke test"}, http.StatusOK, nil)
			var se *statusError
			if errors.As(err, &se) && se.got == http.StatusConflict {
				return nil // registered by an earlier attempt of this run
			}
			return err
		}},
		{"read model " + name + " back", func() error {
			var out struct {
				Model struct {
					Name string `json:"name"`
				} `json:"model"`
			}
			if err := expect(client, "GET", cfg.GatewayURL+"/v1/models/"+name, nil, http.StatusOK, &out); err != nil {
				return err
			}
			if out.Model.Name != name {
				return fmt.Errorf("read back model %q", out.Model.Name)
			}
			return nil
		}},
		{"worker answers a NATS ping", func() error {
			nc, err := nats.Connect(cfg.NATSURL, nats.Name("spotterctl-smoke"), nats.Timeout(3*time.Second))
			if err != nil {
				return err
			}
			defer nc.Close()
			pctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			_, err = natsping.Ping(pctx, nc, "spotterctl-smoke")
			return err
		}},
	}
	for _, c := range checks {
		var last error
		for {
			if last = c.fn(); last == nil {
				passed(c.name)
				break
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("%s: %w", c.name, last)
			case <-time.After(time.Second):
			}
		}
	}
	return nil
}

type statusError struct {
	url       string
	got, want int
	body      string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("%s: status %d, want %d: %s", e.url, e.got, e.want, e.body)
}

func expect(client *http.Client, method, url string, body any, want int, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != want {
		return &statusError{url: url, got: resp.StatusCode, want: want, body: string(raw)}
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}
