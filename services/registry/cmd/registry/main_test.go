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
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

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
	_, err := loadConfig(env(map[string]string{"DATABASE_URL": "postgres://x", "SHUTDOWN_TIMEOUT": "soon"}))
	if err == nil || !strings.Contains(err.Error(), "SHUTDOWN_TIMEOUT") {
		t.Fatalf("err = %v, want an error naming SHUTDOWN_TIMEOUT", err)
	}
}

func TestDefaults(t *testing.T) {
	spectest.Verifies(t, service, "Configuration comes from environment variables")
	c, err := loadConfig(env(map[string]string{"DATABASE_URL": "postgres://x"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":8080" || c.ShutdownTimeout != 10*time.Second {
		t.Fatalf("defaults = %+v", c)
	}
}

// started runs the registry in the background and returns its address and a stop
// function that cancels it and returns run's result.
func started(t *testing.T, dbURL string) (addr string, stop func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, env(map[string]string{"DATABASE_URL": dbURL, "HTTP_ADDR": "127.0.0.1:0", "SHUTDOWN_TIMEOUT": "5s"}),
			quiet, func(a string) { ready <- a })
	}()
	select {
	case addr = <-ready:
	case err := <-done:
		cancel()
		t.Fatalf("registry exited during startup: %v", err)
	case <-time.After(30 * time.Second):
		cancel()
		t.Fatal("registry did not start within 30s")
	}
	var stopped bool
	stop = func() error {
		if stopped {
			return nil
		}
		stopped = true
		cancel()
		select {
		case err := <-done:
			return err
		case <-time.After(10 * time.Second):
			return errors.New("registry did not stop within 10s")
		}
	}
	t.Cleanup(func() { _ = stop() })
	return addr, stop
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

func TestStartMigratesThenServesHealth(t *testing.T) {
	spectest.Verifies(t, service, "Migrations run at startup")
	spectest.Verifies(t, service, "Liveness and readiness are reported over HTTP")
	dbURL := pgtest.NewSchema(t)
	addr, _ := started(t, dbURL)

	for _, path := range []string{"/livez", "/readyz"} {
		if code := get(t, "http://"+addr+path); code != http.StatusOK {
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
	_, stop := started(t, dbURL)
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	// The second start must succeed; that it applies zero migrations is checked by the
	// postgres adapter's TestMigrateTwiceAppliesNothingTheSecondTime.
	_, stop = started(t, dbURL)
	if err := stop(); err != nil {
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

func TestStopSignalShutsDownGracefully(t *testing.T) {
	spectest.Verifies(t, service, "Shutdown is graceful")
	addr, stop := started(t, pgtest.NewSchema(t))
	start := time.Now()
	if err := stop(); err != nil {
		t.Fatalf("run returned %v, want nil", err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("shutdown took %v, longer than SHUTDOWN_TIMEOUT", took)
	}
	if conn, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		conn.Close()
		t.Fatal("port still accepts connections after shutdown")
	}
}
