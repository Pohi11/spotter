// Command gateway runs Spotter's public HTTP front door.
//
// Configuration comes only from environment variables: REGISTRY_ADDR (required, the
// registry's gRPC address), HTTP_ADDR (default :8080), SHUTDOWN_TIMEOUT (default 10s).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/svc"
	"github.com/Pohi11/spotter/services/gateway/internal/restapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv, logger, nil); err != nil {
		logger.Error("gateway stopped", "error", err)
		os.Exit(1)
	}
}

type config struct {
	RegistryAddr    string
	HTTPAddr        string
	ShutdownTimeout time.Duration
}

func loadConfig(getenv func(string) string) (config, error) {
	env := svc.Env(getenv)
	registry, err := env.Required("REGISTRY_ADDR")
	if err != nil {
		return config{}, err
	}
	timeout, err := env.Duration("SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return config{}, err
	}
	return config{RegistryAddr: registry, HTTPAddr: env.Or("HTTP_ADDR", ":8080"), ShutdownTimeout: timeout}, nil
}

// run serves the gateway until ctx is cancelled. onListening receives the bound address.
func run(ctx context.Context, getenv func(string) string, logger *slog.Logger, onListening func(name, addr string)) error {
	cfg, err := loadConfig(getenv)
	if err != nil {
		return err
	}
	// NewClient connects lazily: the gateway starts even if the registry isn't up yet,
	// and /readyz reports it until it is.
	conn, err := grpc.NewClient(cfg.RegistryAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("REGISTRY_ADDR: %w", err)
	}
	defer conn.Close()

	api, err := restapi.New(ctx, conn)
	if err != nil {
		return err
	}
	return svc.Run(ctx, svc.Options{Logger: logger, ShutdownTimeout: cfg.ShutdownTimeout, OnListening: onListening},
		svc.Listener{Name: "http", Addr: cfg.HTTPAddr, Server: svc.HTTP(routes(api, healthpb.NewHealthClient(conn)))})
}

// routes adds liveness and readiness in front of the API.
func routes(api http.Handler, registryHealth healthpb.HealthClient) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})
	// Ready only when the next hop is: the registry's own health check (design D6).
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		resp, err := registryHealth.Check(ctx, &healthpb.HealthCheckRequest{Service: spotterv1.RegistryService_ServiceDesc.ServiceName})
		if err != nil || resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
			http.Error(w, "registry unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.Handle("/", api)
	return mux
}
