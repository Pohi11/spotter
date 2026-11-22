// Command monitor watches inference completions for drift against each model version's
// training-time reference (decision record 0012).
//
// Configuration (environment variables): NATS_URL, REGISTRY_ADDR, S3_ENDPOINT,
// S3_ACCESS_KEY, S3_SECRET_KEY (required); S3_REGION (us-east-1), HTTP_ADDR (:8082),
// SHUTDOWN_TIMEOUT (10s).
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

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/natsjs"
	"github.com/Pohi11/spotter/internal/svc"
	"github.com/Pohi11/spotter/internal/telemetry"
	"github.com/Pohi11/spotter/services/monitor/internal/adapters"
	"github.com/Pohi11/spotter/services/monitor/internal/app"
	"github.com/Pohi11/spotter/services/monitor/internal/domain"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv, logger, nil); err != nil {
		logger.Error("monitor stopped", "error", err)
		os.Exit(1)
	}
}

type config struct {
	NATSURL, RegistryAddr, HTTPAddr string
	S3                              adapters.S3Config
	ShutdownTimeout                 time.Duration
}

func loadConfig(getenv func(string) string) (config, error) {
	env := svc.Env(getenv)
	var c config
	var err error
	for _, req := range []struct {
		key string
		dst *string
	}{
		{"NATS_URL", &c.NATSURL}, {"REGISTRY_ADDR", &c.RegistryAddr},
		{"S3_ENDPOINT", &c.S3.Endpoint}, {"S3_ACCESS_KEY", &c.S3.AccessKey}, {"S3_SECRET_KEY", &c.S3.SecretKey},
	} {
		if *req.dst, err = env.Required(req.key); err != nil {
			return config{}, err
		}
	}
	if c.ShutdownTimeout, err = env.Duration("SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return config{}, err
	}
	c.HTTPAddr = env.Or("HTTP_ADDR", ":8082")
	c.S3.Region = env.Or("S3_REGION", "us-east-1")
	return c, nil
}

func run(ctx context.Context, getenv func(string) string, logger *slog.Logger, onListening func(name, addr string)) error {
	cfg, err := loadConfig(getenv)
	if err != nil {
		return err
	}
	tel, err := telemetry.Setup(ctx, "monitor")
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	defer func() { _ = tel.Shutdown(context.Background()) }()

	nc, err := nats.Connect(cfg.NATSURL, nats.Name("monitor"), nats.MaxReconnects(-1))
	if err != nil {
		return fmt.Errorf("NATS_URL: %w", err)
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}
	if err := natsjs.Ensure(ctx, js); err != nil {
		return fmt.Errorf("ensure streams: %w", err)
	}
	conn, err := grpc.NewClient(cfg.RegistryAddr, grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		return fmt.Errorf("REGISTRY_ADDR: %w", err)
	}
	defer conn.Close()
	metrics, err := adapters.NewMetrics()
	if err != nil {
		return err
	}
	mon := app.NewMonitor(adapters.NewReferences(spotterv1.NewRegistryServiceClient(conn), cfg.S3),
		adapters.Publisher{JS: js}, metrics, systemClock{}, domain.DefaultConfig)

	consumeCtx, stopConsuming := context.WithCancel(ctx)
	consumed := make(chan error, 1)
	go func() { consumed <- adapters.Consume(consumeCtx, js, mon, logger) }()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok\n")) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !nc.IsConnected() {
			http.Error(w, "nats unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.Handle("GET /metrics", tel.Metrics)

	runErr := svc.Run(ctx, svc.Options{Logger: logger, ShutdownTimeout: cfg.ShutdownTimeout, OnListening: onListening},
		svc.Listener{Name: "http", Addr: cfg.HTTPAddr, Server: svc.HTTP(mux)})
	stopConsuming()
	if err := <-consumed; err != nil && runErr == nil {
		runErr = err
	}
	return runErr
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }
