package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/protobuf/proto"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/natsjs"
	"github.com/Pohi11/spotter/internal/natstest"
	"github.com/Pohi11/spotter/internal/pgtest"
	"github.com/Pohi11/spotter/internal/spectest"
)

const service = "registry-service"

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestMissingDatabaseURL(t *testing.T) {
	spectest.Verifies(t, service, "Configuration comes from environment variables")
	err := run(context.Background(), env(nil), quiet, nil)
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("err = %v, want an error naming DATABASE_URL", err)
	}
}

func TestInvalidShutdownTimeout(t *testing.T) {
	spectest.Verifies(t, service, "Configuration comes from environment variables")
	_, err := loadConfig(env(map[string]string{"DATABASE_URL": "postgres://x", "NATS_URL": "nats://x", "SHUTDOWN_TIMEOUT": "soon"}))
	if err == nil || !strings.Contains(err.Error(), "SHUTDOWN_TIMEOUT") {
		t.Fatalf("err = %v, want an error naming SHUTDOWN_TIMEOUT", err)
	}
}

func TestDefaults(t *testing.T) {
	spectest.Verifies(t, service, "Configuration comes from environment variables")
	c, err := loadConfig(env(map[string]string{"DATABASE_URL": "postgres://x", "NATS_URL": "nats://x"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":8080" || c.GRPCAddr != ":9090" || c.ShutdownTimeout != 10*time.Second {
		t.Fatalf("defaults = %+v", c)
	}
}

type registry struct {
	http, grpc string
	nats       *natstest.Server
	stop       func() error
}

// started runs the registry in the background on ports chosen by the OS, with its own
// embedded NATS server.
func started(t *testing.T, dbURL string) registry {
	t.Helper()
	ns := natstest.Start(t)
	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	addrs := map[string]string{}
	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, env(map[string]string{
			"DATABASE_URL": dbURL, "NATS_URL": ns.URL, "HTTP_ADDR": "127.0.0.1:0", "GRPC_ADDR": "127.0.0.1:0", "SHUTDOWN_TIMEOUT": "5s",
		}), quiet, func(name, addr string) {
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
		t.Fatalf("registry exited during startup: %v", err)
	case <-time.After(30 * time.Second):
		cancel()
		t.Fatal("registry did not start within 30s")
	}
	var once sync.Once
	var stopErr error
	stop := func() error {
		once.Do(func() {
			cancel()
			select {
			case stopErr = <-done:
			case <-time.After(10 * time.Second):
				stopErr = errors.New("registry did not stop within 10s")
			}
		})
		return stopErr
	}
	t.Cleanup(func() { _ = stop() })
	return registry{http: addrs["http"], grpc: addrs["grpc"], nats: ns, stop: stop}
}

func get(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func dial(t *testing.T, addr string) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestStartMigratesThenServesHealth(t *testing.T) {
	spectest.Verifies(t, service, "Migrations run at startup")
	spectest.Verifies(t, service, "Liveness and readiness are reported over HTTP")
	dbURL := pgtest.NewSchema(t)
	r := started(t, dbURL)

	for _, path := range []string{"/livez", "/readyz"} {
		if code := get(t, "http://"+r.http+path); code != http.StatusOK {
			t.Fatalf("%s = %d, want 200", path, code)
		}
	}
	conn, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	var tables int
	if err := conn.QueryRow(context.Background(),
		`SELECT count(*) FROM information_schema.tables WHERE table_schema = current_schema()
		 AND table_name IN ('models', 'model_versions', 'stage_changes', 'outbox')`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 4 {
		t.Fatalf("found %d of 4 registry tables after startup", tables)
	}
}

func TestSecondStartAppliesNothing(t *testing.T) {
	spectest.Verifies(t, service, "Migrations run at startup")
	dbURL := pgtest.NewSchema(t)
	if err := started(t, dbURL).stop(); err != nil {
		t.Fatal(err)
	}
	// The second start must succeed; that it applies zero migrations is checked by the
	// postgres adapter's TestMigrateTwiceAppliesNothingTheSecondTime.
	if err := started(t, dbURL).stop(); err != nil {
		t.Fatalf("second start: %v", err)
	}
}

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func TestReadinessFollowsTheDatabase(t *testing.T) {
	spectest.Verifies(t, service, "Liveness and readiness are reported over HTTP")
	srv := httptest.NewServer(healthHandler(fakePinger{err: errors.New("connection refused")}))
	defer srv.Close()
	if code := get(t, srv.URL+"/readyz"); code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz = %d with database down, want 503", code)
	}
	if code := get(t, srv.URL+"/livez"); code != http.StatusOK {
		t.Fatalf("/livez = %d with database down, want 200", code)
	}
}

func TestGRPCHealthAndReflection(t *testing.T) {
	spectest.Verifies(t, "registry-api", "Health and reflection are served over gRPC")
	r := started(t, pgtest.NewSchema(t))
	conn := dial(t, r.grpc)
	ctx := context.Background()

	resp, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{Service: spotterv1.RegistryService_ServiceDesc.ServiceName})
	if err != nil || resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("health = %v, %v; want SERVING", resp, err)
	}

	stream, err := reflectionpb.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Send(&reflectionpb.ServerReflectionRequest{MessageRequest: &reflectionpb.ServerReflectionRequest_ListServices{}}); err != nil {
		t.Fatal(err)
	}
	reply, err := stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range reply.GetListServicesResponse().GetService() {
		names = append(names, s.GetName())
	}
	if !strings.Contains(strings.Join(names, ","), "spotter.v1.RegistryService") {
		t.Fatalf("reflection lists %v", names)
	}
}

func TestGRPCServesRegistryOperations(t *testing.T) {
	spectest.Verifies(t, "registry-api", "Registry operations are available over gRPC")
	r := started(t, pgtest.NewSchema(t))
	c := spotterv1.NewRegistryServiceClient(dial(t, r.grpc))
	if _, err := c.RegisterModel(context.Background(), &spotterv1.RegisterModelRequest{Name: "eurosat-classifier"}); err != nil {
		t.Fatal(err)
	}
	m, err := c.GetModel(context.Background(), &spotterv1.GetModelRequest{Name: "eurosat-classifier"})
	if err != nil || m.GetModel().GetName() != "eurosat-classifier" {
		t.Fatalf("get model over a real port against Postgres: %v, %v", m, err)
	}
}

func TestStopSignalShutsDownGracefully(t *testing.T) {
	spectest.Verifies(t, service, "Shutdown is graceful")
	r := started(t, pgtest.NewSchema(t))
	start := time.Now()
	if err := r.stop(); err != nil {
		t.Fatalf("run returned %v, want nil", err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("shutdown took %v, longer than SHUTDOWN_TIMEOUT", took)
	}
	for name, addr := range map[string]string{"http": r.http, "grpc": r.grpc} {
		if conn, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
			conn.Close()
			t.Fatalf("%s port still accepts connections after shutdown", name)
		}
	}
}

func TestMissingNATSURL(t *testing.T) {
	spectest.Verifies(t, service, "Configuration comes from environment variables")
	_, err := loadConfig(env(map[string]string{"DATABASE_URL": "postgres://x"}))
	if err == nil || !strings.Contains(err.Error(), "NATS_URL") {
		t.Fatalf("err = %v, want an error naming NATS_URL", err)
	}
}

func TestTransitionReachesTheStream(t *testing.T) {
	spectest.Verifies(t, "event-streams", "The registry publishes its outbox")
	dbURL := pgtest.NewSchema(t)
	r := started(t, dbURL)
	ctx := context.Background()
	c := spotterv1.NewRegistryServiceClient(dial(t, r.grpc))
	if _, err := c.RegisterModel(ctx, &spotterv1.RegisterModelRequest{Name: "eurosat-classifier"}); err != nil {
		t.Fatal(err)
	}
	v, err := c.RegisterVersion(ctx, &spotterv1.RegisterVersionRequest{ModelName: "eurosat-classifier", ArtifactUri: "s3://m/k",
		ArtifactSha256: "6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b", Framework: "pytorch"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.TransitionVersion(ctx, &spotterv1.TransitionVersionRequest{VersionId: v.GetVersion().GetId(),
		ToStage: spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_VALIDATED, Reason: "eval passed", ExpectedRevision: 1}); err != nil {
		t.Fatal(err)
	}

	_, js := r.nats.Connect(t)
	var event spotterv1.ModelVersionStageChanged
	deadline := time.Now().Add(10 * time.Second)
	for {
		s, err := js.Stream(ctx, natsjs.RegistryStream)
		if err == nil {
			if msg, err := s.GetLastMsgForSubject(ctx, "registry.events.model_version_stage_changed"); err == nil {
				if err := proto.Unmarshal(msg.Data, &event); err != nil {
					t.Fatal(err)
				}
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("no stage-changed event reached the stream within 10s")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if event.GetVersionId() != v.GetVersion().GetId() || event.GetToStage() != spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_VALIDATED {
		t.Fatalf("event = %v", &event)
	}

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	deadline = time.Now().Add(5 * time.Second)
	for {
		var pending int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE published_at IS NULL`).Scan(&pending); err != nil {
			t.Fatal(err)
		}
		if pending == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d outbox records still unpublished", pending)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
