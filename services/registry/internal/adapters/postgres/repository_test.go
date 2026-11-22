package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pohi11/spotter/internal/pgtest"
	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/services/registry/internal/adapters/postgres"
	"github.com/Pohi11/spotter/services/registry/internal/ports"
	"github.com/Pohi11/spotter/services/registry/internal/registrytest"
)

// newPool returns a pool on a fresh, migrated schema.
func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, pgtest.NewSchema(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

func TestContract(t *testing.T) {
	registrytest.Run(t, func(t *testing.T) ports.Repository { return postgres.New(newPool(t)) })
}

// The partial unique index must hold even when application code is bypassed.
func TestDatabaseRejectsSecondProductionRow(t *testing.T) {
	spectest.Verifies(t, "model-lifecycle", "At most one production version per model")
	ctx := context.Background()
	pool := newPool(t)

	modelID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO models (id, name, created_at) VALUES ($1, 'eurosat-classifier', now())`, modelID); err != nil {
		t.Fatal(err)
	}
	insert := func(number int) error {
		_, err := pool.Exec(ctx, `
			INSERT INTO model_versions (id, model_id, version, artifact_uri, artifact_sha256, framework, stage, revision, created_at, updated_at)
			VALUES ($1, $2, $3, 's3://m/k', $4, 'pytorch', 'production', 1, now(), now())`,
			uuid.New(), modelID, number, registrytest.SHA)
		return err
	}
	if err := insert(1); err != nil {
		t.Fatalf("first production row: %v", err)
	}
	if err := insert(2); err == nil {
		t.Fatal("database accepted a second production row for the same model")
	}
}

func TestMigrateTwiceAppliesNothingTheSecondTime(t *testing.T) {
	pool := newPool(t) // migrated once
	n, err := postgres.Migrate(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("second migration applied %d migrations, want 0", n)
	}
}
