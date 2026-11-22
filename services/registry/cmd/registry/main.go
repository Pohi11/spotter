// Command registry runs the Spotter model registry.
//
// Configuration comes only from environment variables: DATABASE_URL (required),
// NATS_URL (required), HTTP_ADDR (default :8080), GRPC_ADDR (default :9090),
// SHUTDOWN_TIMEOUT (default 10s).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/natsjs"
	"github.com/Pohi11/spotter/internal/svc"
	"github.com/Pohi11/spotter/services/registry/internal/adapters/grpcapi"
	"github.com/Pohi11/spotter/services/registry/internal/adapters/natspub"
	"github.com/Pohi11/spotter/services/registry/internal/adapters/postgres"
	"github.com/Pohi11/spotter/services/registry/internal/adapters/protoevents"
	"github.com/Pohi11/spotter/services/registry/internal/app"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	// SIGINT (Ctrl+C) and SIGTERM (Kubernetes stopping the pod) both cancel ctx.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Getenv, logger, nil); err != nil {
		logger.Error("registry stopped", "error", err)
		os.Exit(1)
	}
}

// run starts the registry and blocks until ctx is cancelled, then shuts down gracefully.
// onListening, if set, receives each bound address ("http", "grpc"); tests use it with
// port 0.
func run(ctx context.Context, getenv func(string) string, logger *slog.Logger, onListening func(name, addr string)) error {
	cfg, err := loadConfig(getenv)
	if err != nil {
		return err
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("DATABASE_URL: %w", err)
	}
	defer pool.Close()

	applied, err := postgres.Migrate(ctx, pool)
	if err != nil {
		return err
	}
	logger.Info("schema up to date", "migrations_applied", applied)

	// Keep reconnecting forever: while NATS is away the relay just leaves records pending.
	nc, err := nats.Connect(cfg.NATSURL, nats.Name("registry"), nats.MaxReconnects(-1))
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

	repo := postgres.New(pool)
	service := app.NewService(repo, protoevents.Encoder{}, uuidV7{}, systemClock{})

	// The outbox relay runs until ctx is cancelled; run waits for it before returning.
	relayDone := make(chan struct{})
	go func() {
		defer close(relayDone)
		relayOutbox(ctx, app.NewRelay(repo, natspub.Publisher{JS: js}, systemClock{}), logger)
	}()
	defer func() { <-relayDone }()

	grpcServer := grpc.NewServer()
	spotterv1.RegisterRegistryServiceServer(grpcServer, grpcapi.New(service, logger))
	healthServer := health.NewServer()
	healthServer.SetServingStatus(spotterv1.RegistryService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	reflection.Register(grpcServer)

	return svc.Run(ctx, svc.Options{
		Logger:          logger,
		ShutdownTimeout: cfg.ShutdownTimeout,
		OnListening:     onListening,
		// Report NOT_SERVING first so load balancers stop routing before connections close.
		BeforeShutdown: healthServer.Shutdown,
	},
		svc.Listener{Name: "http", Addr: cfg.HTTPAddr, Server: svc.HTTP(healthHandler(pool))},
		svc.Listener{Name: "grpc", Addr: cfg.GRPCAddr, Server: svc.GRPC(grpcServer)},
	)
}

// relayOutbox publishes pending outbox records every relayInterval until ctx is done.
func relayOutbox(ctx context.Context, relay *app.Relay, logger *slog.Logger) {
	ticker := time.NewTicker(relayInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := relay.Once(ctx, 100); err != nil && ctx.Err() == nil {
				logger.Warn("outbox relay", "published", n, "error", err)
			}
		}
	}
}

const relayInterval = 500 * time.Millisecond

type uuidV7 struct{}

func (uuidV7) NewID() uuid.UUID { return uuid.Must(uuid.NewV7()) }

type systemClock struct{}

// Now is truncated to microseconds, the precision Postgres stores, so a returned value
// equals the same value read back later.
func (systemClock) Now() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }
