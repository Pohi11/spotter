// Package pgtest gives tests a real, empty Postgres schema (decision record 0005).
//
// The first call in a test binary starts one Postgres container with testcontainers;
// every call creates a fresh schema in it and returns a connection string whose
// search_path points at that schema, so tests can run in parallel without seeing
// each other's rows. Set PGTEST_DATABASE_URL to use an existing server instead.
package pgtest

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// Image is the Postgres image tests run against; it matches deploy/compose.
const Image = "postgres:18"

var (
	once    sync.Once
	baseURL string
	initErr error
)

func server() (string, error) {
	once.Do(func() {
		if u := os.Getenv("PGTEST_DATABASE_URL"); u != "" {
			baseURL = u
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		c, err := tcpostgres.Run(ctx, Image,
			tcpostgres.WithDatabase("spotter_test"),
			tcpostgres.WithUsername("spotter"),
			tcpostgres.WithPassword("spotter-test"),
			tcpostgres.BasicWaitStrategies(),
		)
		if err != nil {
			initErr = fmt.Errorf("start postgres container: %w", err)
			return
		}
		// The container is removed by testcontainers' reaper when the test binary exits.
		baseURL, initErr = c.ConnectionString(ctx, "sslmode=disable")
	})
	return baseURL, initErr
}

// NewSchema creates an empty schema for this test, drops it when the test ends, and
// returns a connection string that uses it.
func NewSchema(t testing.TB) string {
	t.Helper()
	base, err := server()
	if err != nil {
		t.Fatalf("pgtest: %v", err)
	}
	schema := "t_" + strings.ReplaceAll(uuid.NewString(), "-", "")

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("pgtest: connect: %v", err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("pgtest: create schema: %v", err)
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(context.Background(), base)
		if err != nil {
			t.Errorf("pgtest: cleanup connect: %v", err)
			return
		}
		defer c.Close(context.Background())
		if _, err := c.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("pgtest: drop schema: %v", err)
		}
	})

	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("pgtest: parse url: %v", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String()
}
