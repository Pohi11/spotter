package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/services/registry/registryfake"
)

const capability = "gateway-http"

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func env(vars map[string]string) func(string) string { return func(k string) string { return vars[k] } }

func TestMissingRegistryAddr(t *testing.T) {
	spectest.Verifies(t, capability, "The gateway process is configured from the environment and stops gracefully")
	err := run(context.Background(), env(nil), quiet, nil)
	if err == nil || !strings.Contains(err.Error(), "REGISTRY_ADDR") {
		t.Fatalf("err = %v, want an error naming REGISTRY_ADDR", err)
	}
}

// started runs the gateway against registryAddr and returns its address and a stop func.
func started(t *testing.T, registryAddr string) (string, func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, env(map[string]string{"REGISTRY_ADDR": registryAddr, "HTTP_ADDR": "127.0.0.1:0", "SHUTDOWN_TIMEOUT": "5s"}),
			quiet, func(_, addr string) { ready <- addr })
	}()
	var addr string
	select {
	case addr = <-ready:
	case err := <-done:
		cancel()
		t.Fatalf("gateway exited during startup: %v", err)
	}
	var once sync.Once
	var stopErr error
	stop := func() error {
		once.Do(func() {
			cancel()
			select {
			case stopErr = <-done:
			case <-time.After(10 * time.Second):
				stopErr = errors.New("gateway did not stop within 10s")
			}
		})
		return stopErr
	}
	t.Cleanup(func() { _ = stop() })
	return addr, stop
}

func status(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestReadinessFollowsTheRegistry(t *testing.T) {
	spectest.Verifies(t, capability, "The gateway process is configured from the environment and stops gracefully")
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	reg := registryfake.Serve(lis)
	addr, _ := started(t, lis.Addr().String())

	if code := status(t, "http://"+addr+"/readyz"); code != http.StatusOK {
		t.Fatalf("/readyz with registry SERVING = %d, want 200", code)
	}
	reg.Health.SetServingStatus(spotterv1.RegistryService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
	if code := status(t, "http://"+addr+"/readyz"); code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz with registry NOT_SERVING = %d, want 503", code)
	}
	reg.GRPC.Stop()
	if code := status(t, "http://"+addr+"/readyz"); code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz with registry stopped = %d, want 503", code)
	}
	if code := status(t, "http://"+addr+"/livez"); code != http.StatusOK {
		t.Fatalf("/livez = %d, want 200 throughout", code)
	}
}

func TestStopSignal(t *testing.T) {
	spectest.Verifies(t, capability, "The gateway process is configured from the environment and stops gracefully")
	addr, stop := started(t, "127.0.0.1:1")
	if err := stop(); err != nil {
		t.Fatalf("run returned %v, want nil", err)
	}
	if conn, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		conn.Close()
		t.Fatal("port still accepts connections after shutdown")
	}
}

func TestRESTThroughTheProcess(t *testing.T) {
	spectest.Verifies(t, capability, "Registry operations are available over REST")
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	reg := registryfake.Serve(lis)
	t.Cleanup(reg.GRPC.Stop)
	addr, _ := started(t, lis.Addr().String())
	resp, err := http.Post("http://"+addr+"/v1/models", "application/json", strings.NewReader(`{"name":"eurosat-classifier"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /v1/models through the gateway process = %d", resp.StatusCode)
	}
}
