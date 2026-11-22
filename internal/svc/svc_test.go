package svc_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/Pohi11/spotter/internal/svc"
)

func env(m map[string]string) svc.Env { return func(k string) string { return m[k] } }

func TestEnv(t *testing.T) {
	e := env(map[string]string{"A": "x", "T": "250ms", "BAD": "soon"})
	if _, err := e.Required("MISSING"); err == nil || !strings.Contains(err.Error(), "MISSING") {
		t.Errorf("Required(MISSING) err = %v", err)
	}
	if v := e.Or("A", "d"); v != "x" {
		t.Errorf("Or(A) = %q", v)
	}
	if v := e.Or("UNSET", "d"); v != "d" {
		t.Errorf("Or(UNSET) = %q", v)
	}
	if d, err := e.Duration("T", time.Second); err != nil || d != 250*time.Millisecond {
		t.Errorf("Duration(T) = %v, %v", d, err)
	}
	if _, err := e.Duration("BAD", time.Second); err == nil || !strings.Contains(err.Error(), "BAD") {
		t.Errorf("Duration(BAD) err = %v", err)
	}
}

func TestRunServesThenShutsDownEverything(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	addrs := map[string]string{}
	ready := make(chan struct{})
	beforeShutdown := false
	done := make(chan error, 1)
	go func() {
		done <- svc.Run(ctx, svc.Options{
			Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
			ShutdownTimeout: 2 * time.Second,
			OnListening: func(name, addr string) {
				addrs[name] = addr
				if len(addrs) == 2 {
					close(ready)
				}
			},
			BeforeShutdown: func() { beforeShutdown = true },
		},
			svc.Listener{Name: "http", Addr: "127.0.0.1:0", Server: svc.HTTP(http.NotFoundHandler())},
			svc.Listener{Name: "grpc", Addr: "127.0.0.1:0", Server: svc.GRPC(grpc.NewServer())},
		)
	}()
	<-ready
	for name, addr := range addrs {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("%s not accepting: %v", name, err)
		}
		c.Close()
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run = %v, want nil", err)
	}
	if !beforeShutdown {
		t.Error("BeforeShutdown was not called")
	}
	for name, addr := range addrs {
		if c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond); err == nil {
			c.Close()
			t.Errorf("%s still accepting after shutdown", name)
		}
	}
}

func TestRunFailsWhenAnAddressIsTaken(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	err = svc.Run(context.Background(), svc.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), ShutdownTimeout: time.Second},
		svc.Listener{Name: "http", Addr: taken.Addr().String(), Server: svc.HTTP(http.NotFoundHandler())})
	if err == nil || !strings.Contains(err.Error(), "http listen") {
		t.Fatalf("err = %v, want a listen error", err)
	}
}
