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

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/natstest"
	"github.com/Pohi11/spotter/internal/s3test"
	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/services/registry/registryfake"
)

const capability = "gateway-http"

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func env(vars map[string]string) func(string) string { return func(k string) string { return vars[k] } }

// baseEnv is a complete configuration pointing at a fresh NATS server and the test S3 store.
func baseEnv(t *testing.T, registryAddr string) (map[string]string, *natstest.Server) {
	t.Helper()
	ns := natstest.Start(t)
	st := s3test.Start(t)
	return map[string]string{
		"REGISTRY_ADDR": registryAddr, "NATS_URL": ns.URL,
		"S3_ENDPOINT": st.Endpoint, "S3_ACCESS_KEY": st.AccessKey, "S3_SECRET_KEY": st.SecretKey,
		"IMAGES_BUCKET": s3test.BucketName(),
		"HTTP_ADDR":     "127.0.0.1:0", "GRPC_ADDR": "127.0.0.1:0", "SHUTDOWN_TIMEOUT": "5s",
	}, ns
}

func TestMissingRegistryAddr(t *testing.T) {
	spectest.Verifies(t, capability, "The gateway process is configured from the environment and stops gracefully")
	err := run(context.Background(), env(nil), quiet, nil)
	if err == nil || !strings.Contains(err.Error(), "REGISTRY_ADDR") {
		t.Fatalf("err = %v, want an error naming REGISTRY_ADDR", err)
	}
}

func TestMissingNATSURL(t *testing.T) {
	spectest.Verifies(t, capability, "The gateway process is configured from the environment and stops gracefully")
	_, err := loadConfig(env(map[string]string{"REGISTRY_ADDR": "localhost:9090"}))
	if err == nil || !strings.Contains(err.Error(), "NATS_URL") {
		t.Fatalf("err = %v, want an error naming NATS_URL", err)
	}
}

// started runs the gateway with vars and returns its HTTP address and a stop func.
func started(t *testing.T, vars map[string]string) (map[string]string, func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	addrs := map[string]string{}
	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, env(vars), quiet, func(name, addr string) {
			mu.Lock()
			defer mu.Unlock()
			addrs[name] = addr
			if len(addrs) == 2 {
				close(ready)
			}
		})
	}()
	select {
	case <-ready:
	case err := <-done:
		cancel()
		t.Fatalf("gateway exited during startup: %v", err)
	case <-time.After(60 * time.Second):
		cancel()
		t.Fatal("gateway did not start within 60s")
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
	return addrs, stop
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

func fakeRegistry(t *testing.T) (*registryfake.Server, string) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	reg := registryfake.Serve(lis)
	t.Cleanup(reg.GRPC.Stop)
	return reg, lis.Addr().String()
}

func TestReadinessFollowsTheRegistry(t *testing.T) {
	spectest.Verifies(t, capability, "The gateway process is configured from the environment and stops gracefully")
	reg, regAddr := fakeRegistry(t)
	vars, _ := baseEnv(t, regAddr)
	a, _ := started(t, vars)
	addr := a["http"]

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

func TestReadinessFollowsNATS(t *testing.T) {
	spectest.Verifies(t, capability, "The gateway process is configured from the environment and stops gracefully")
	_, regAddr := fakeRegistry(t)
	vars, ns := baseEnv(t, regAddr)
	a, _ := started(t, vars)
	addr := a["http"]
	if code := status(t, "http://"+addr+"/readyz"); code != http.StatusOK {
		t.Fatalf("/readyz = %d, want 200", code)
	}
	ns.Shutdown()
	deadline := time.Now().Add(5 * time.Second)
	for status(t, "http://"+addr+"/readyz") != http.StatusServiceUnavailable {
		if time.Now().After(deadline) {
			t.Fatal("/readyz still 200 with NATS down")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestStopSignal(t *testing.T) {
	spectest.Verifies(t, capability, "The gateway process is configured from the environment and stops gracefully")
	_, regAddr := fakeRegistry(t)
	vars, _ := baseEnv(t, regAddr)
	a, stop := started(t, vars)
	addr := a["http"]
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
	_, regAddr := fakeRegistry(t)
	vars, _ := baseEnv(t, regAddr)
	a, _ := started(t, vars)
	addr := a["http"]
	resp, err := http.Post("http://"+addr+"/v1/models", "application/json", strings.NewReader(`{"name":"eurosat-classifier"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /v1/models through the gateway process = %d", resp.StatusCode)
	}
	// The inference REST path is served in process.
	resp, err = http.Post("http://"+addr+"/v1/models/eurosat-classifier/inferences", "application/json", strings.NewReader(`{"image":"aGVsbG8="}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST inferences through the gateway process = %d", resp.StatusCode)
	}
}

func TestMetricsAfterGRPCCalls(t *testing.T) {
	spectest.Verifies(t, "observability", "Services expose Prometheus metrics")
	_, regAddr := fakeRegistry(t)
	vars, _ := baseEnv(t, regAddr)
	a, _ := started(t, vars)

	conn, err := grpc.NewClient(a["grpc"], grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := spotterv1.NewInferenceServiceClient(conn)
	for range 3 { // unknown jobs: NOT_FOUND still counts as a served call
		_, _ = client.GetInference(context.Background(), &spotterv1.GetInferenceRequest{JobId: "00000000-0000-0000-0000-000000000000"})
	}

	resp, err := http.Get("http://" + a["http"] + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var count string
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "rpc_server_call_duration_seconds_count") && strings.Contains(line, "GetInference") {
			count = line[strings.LastIndex(line, " ")+1:]
		}
	}
	if count == "" || count == "0" {
		t.Fatalf("no non-zero gRPC server duration count for GetInference in /metrics:\n%.2000s", body)
	}
}
