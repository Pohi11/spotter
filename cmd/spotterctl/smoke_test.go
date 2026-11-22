package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/natsping"
	"github.com/Pohi11/spotter/internal/natstest"
	"github.com/Pohi11/spotter/internal/spectest"
)

const install = "The platform installs into an empty cluster"

// fakePlatform serves /readyz and the gateway's model endpoints; readyAfter makes the gateway
// report 503 for its first n readiness checks.
func fakePlatform(t *testing.T, readyAfter int) (gateway, registry *httptest.Server) {
	t.Helper()
	var mu sync.Mutex
	models := map[string]bool{}
	checks := 0
	gw := http.NewServeMux()
	gw.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if checks++; checks <= readyAfter {
			http.Error(w, "starting", http.StatusServiceUnavailable)
		}
	})
	gw.HandleFunc("POST /v1/models", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Name string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		mu.Lock()
		models[in.Name] = true
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"model": map[string]string{"name": in.Name}})
	})
	gw.HandleFunc("GET /v1/models/{name}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		ok := models[r.PathValue("name")]
		mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": map[string]string{"name": r.PathValue("name")}})
	})
	reg := http.NewServeMux()
	reg.HandleFunc("GET /readyz", func(http.ResponseWriter, *http.Request) {})
	gateway, registry = httptest.NewServer(gw), httptest.NewServer(reg)
	t.Cleanup(gateway.Close)
	t.Cleanup(registry.Close)
	return gateway, registry
}

func respondToPings(t *testing.T, nc *nats.Conn) {
	t.Helper()
	sub, err := nc.Subscribe(natsping.Subject, func(m *nats.Msg) {
		var req spotterv1.PingRequest
		_ = proto.Unmarshal(m.Data, &req)
		out, _ := proto.Marshal(&spotterv1.PingReply{Id: req.GetId(), Responder: "fake-worker"})
		_ = m.Respond(out)
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Unsubscribe() })
}

func TestSmokePassesAgainstAWorkingPlatform(t *testing.T) {
	spectest.Verifies(t, "platform-deployment", install)
	srv := natstest.Start(t)
	nc, _ := srv.Connect(t)
	respondToPings(t, nc)
	gw, reg := fakePlatform(t, 2) // not ready at first: the smoke test must wait

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var passed []string
	err := runSmoke(ctx, smokeConfig{GatewayURL: gw.URL, RegistryURL: reg.URL, NATSURL: nc.ConnectedUrl()}, func(s string) { passed = append(passed, s) })
	if err != nil {
		t.Fatal(err)
	}
	if len(passed) != 5 {
		t.Fatalf("passed %v, want 5 checks", passed)
	}
}

func TestSmokeFailsWithoutAWorker(t *testing.T) {
	spectest.Verifies(t, "platform-deployment", install)
	srv := natstest.Start(t)
	nc, _ := srv.Connect(t) // nobody answers pings
	gw, reg := fakePlatform(t, 0)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := runSmoke(ctx, smokeConfig{GatewayURL: gw.URL, RegistryURL: reg.URL, NATSURL: nc.ConnectedUrl()}, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "NATS ping") {
		t.Fatalf("err = %v, want the ping check to fail", err)
	}
}

func TestWaitReturnsOnceTheAddressListens(t *testing.T) {
	spectest.Verifies(t, "platform-deployment", install)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := lis.Addr().String()
	lis.Close() // nothing listens yet

	go func() {
		time.Sleep(1500 * time.Millisecond)
		l, err := net.Listen("tcp", addr)
		if err == nil {
			t.Cleanup(func() { l.Close() })
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start := time.Now()
	if err := waitTCP(ctx, []string{addr}, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < time.Second {
		t.Fatal("returned before the address listened")
	}

	short, cancel2 := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel2()
	if err := waitTCP(short, []string{"127.0.0.1:1"}, func(string) {}); err == nil {
		t.Fatal("waiting for a closed port succeeded")
	}
}
