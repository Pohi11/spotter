// Command gateway runs Spotter's public front door: REST for the registry and the
// inference API over gRPC and REST.
//
// Configuration (environment variables): REGISTRY_ADDR, NATS_URL, S3_ENDPOINT,
// S3_ACCESS_KEY, S3_SECRET_KEY (required); HTTP_ADDR (:8080), GRPC_ADDR (:9091),
// S3_REGION (us-east-1), IMAGES_BUCKET (images), SHUTDOWN_TIMEOUT (10s), and WORKER_ADDR
// (unset: synchronous classification off; the failure lab's baseline).
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

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/natsjs"
	"github.com/Pohi11/spotter/internal/svc"
	"github.com/Pohi11/spotter/internal/telemetry"
	"github.com/Pohi11/spotter/services/gateway/internal/adapters/grpcapi"
	"github.com/Pohi11/spotter/services/gateway/internal/adapters/natsevents"
	"github.com/Pohi11/spotter/services/gateway/internal/adapters/natsjobs"
	"github.com/Pohi11/spotter/services/gateway/internal/adapters/s3images"
	"github.com/Pohi11/spotter/services/gateway/internal/app"
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
	RegistryAddr, NATSURL, HTTPAddr, GRPCAddr string
	WorkerAddr                                string
	S3                                        s3images.Config
	ShutdownTimeout                           time.Duration
}

func loadConfig(getenv func(string) string) (config, error) {
	env := svc.Env(getenv)
	var c config
	var err error
	for _, req := range []struct {
		key string
		dst *string
	}{
		{"REGISTRY_ADDR", &c.RegistryAddr}, {"NATS_URL", &c.NATSURL},
		{"S3_ENDPOINT", &c.S3.Endpoint}, {"S3_ACCESS_KEY", &c.S3.AccessKey}, {"S3_SECRET_KEY", &c.S3.SecretKey},
	} {
		if *req.dst, err = env.Required(req.key); err != nil {
			return config{}, err
		}
	}
	if c.ShutdownTimeout, err = env.Duration("SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return config{}, err
	}
	c.HTTPAddr = env.Or("HTTP_ADDR", ":8080")
	c.GRPCAddr = env.Or("GRPC_ADDR", ":9091")
	c.S3.Region = env.Or("S3_REGION", "us-east-1")
	c.S3.Bucket = env.Or("IMAGES_BUCKET", "images")
	c.WorkerAddr = env.Or("WORKER_ADDR", "")
	return c, nil
}

// run serves the gateway until ctx is cancelled. onListening receives the bound addresses.
func run(ctx context.Context, getenv func(string) string, logger *slog.Logger, onListening func(name, addr string)) error {
	cfg, err := loadConfig(getenv)
	if err != nil {
		return err
	}
	tel, err := telemetry.Setup(ctx, "gateway")
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	defer func() { _ = tel.Shutdown(context.Background()) }()
	// NewClient connects lazily: the gateway starts even if the registry isn't up yet,
	// and /readyz reports it until it is.
	registryConn, err := grpc.NewClient(cfg.RegistryAddr, grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler())) // passes trace context to the registry
	if err != nil {
		return fmt.Errorf("REGISTRY_ADDR: %w", err)
	}
	defer registryConn.Close()

	nc, err := nats.Connect(cfg.NATSURL, nats.Name("gateway"), nats.MaxReconnects(-1))
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
	jobs, err := natsjobs.New(ctx, js)
	if err != nil {
		return err
	}
	images := s3images.New(cfg.S3)
	if err := images.EnsureBucket(ctx); err != nil {
		return fmt.Errorf("S3_ENDPOINT: %w", err)
	}

	deps := app.Deps{
		Images: images, Jobs: jobs, Pub: jobs,
		Models: grpcapi.Catalog{Registry: spotterv1.NewRegistryServiceClient(registryConn)},
		IDs:    uuidV7{}, Clock: systemClock{},
	}
	if cfg.WorkerAddr != "" {
		workerConn, err := grpc.NewClient(cfg.WorkerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
		if err != nil {
			return fmt.Errorf("WORKER_ADDR: %w", err)
		}
		defer workerConn.Close()
		deps.Classifier = grpcapi.Worker{Client: spotterv1.NewClassifierServiceClient(workerConn)}
	}
	service := app.NewService(deps)
	inference := grpcapi.New(service, logger)

	grpcServer := grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
	spotterv1.RegisterInferenceServiceServer(grpcServer, inference)
	healthServer := health.NewServer()
	healthServer.SetServingStatus(spotterv1.InferenceService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	reflection.Register(grpcServer)

	api, err := restapi.New(ctx, registryConn, inference)
	if err != nil {
		return err
	}
	return svc.Run(ctx, svc.Options{Logger: logger, ShutdownTimeout: cfg.ShutdownTimeout, OnListening: onListening, BeforeShutdown: healthServer.Shutdown},
		svc.Listener{Name: "http", Addr: cfg.HTTPAddr, Server: svc.HTTP(routes(otelhttp.NewHandler(api, "gateway"), restapi.Events(natsevents.New(js), restapi.KeepAlive), tel.Metrics, healthpb.NewHealthClient(registryConn), nc))},
		svc.Listener{Name: "grpc", Addr: cfg.GRPCAddr, Server: svc.GRPC(grpcServer)},
	)
}

// routes adds liveness, readiness, metrics, and the event feed in front of the API. The
// feed sits outside the tracing wrapper so a long-lived stream isn't one span.
func routes(api, events, metrics http.Handler, registryHealth healthpb.HealthClient, nc *nats.Conn) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})
	// Ready only when its dependencies are: the registry's health check and NATS.
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if !nc.IsConnected() {
			http.Error(w, "nats unavailable", http.StatusServiceUnavailable)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		resp, err := registryHealth.Check(ctx, &healthpb.HealthCheckRequest{Service: spotterv1.RegistryService_ServiceDesc.ServiceName})
		if err != nil || resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
			http.Error(w, "registry unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.Handle("GET /metrics", metrics)
	mux.Handle("GET /v1/events", events)
	mux.Handle("/", api)
	return mux
}

type uuidV7 struct{}

func (uuidV7) NewID() uuid.UUID { return uuid.Must(uuid.NewV7()) }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }
