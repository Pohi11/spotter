package postgres

import (
	"context"
	"embed"
	"fmt"
	"hash/fnv"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate brings the schema up to date and reports how many migrations it applied.
//
// A Postgres advisory lock lets only one replica apply migrations at a time. The lock id
// is derived from the target schema, so migrating one schema never waits on another
// (tests migrate many schemas in parallel in one database).
func Migrate(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	fsys, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return 0, err
	}
	var schema string
	if err := pool.QueryRow(ctx, `SELECT current_schema()`).Scan(&schema); err != nil {
		return 0, fmt.Errorf("read current schema: %w", err)
	}
	h := fnv.New64a()
	h.Write([]byte("spotter-registry-migrations:" + schema))
	locker, err := lock.NewPostgresSessionLocker(lock.WithLockID(int64(h.Sum64())))
	if err != nil {
		return 0, err
	}
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	p, err := goose.NewProvider(goose.DialectPostgres, db, fsys, goose.WithSessionLocker(locker))
	if err != nil {
		return 0, fmt.Errorf("migrations: %w", err)
	}
	results, err := p.Up(ctx)
	if err != nil {
		return 0, fmt.Errorf("migrate: %w", err)
	}
	return len(results), nil
}
