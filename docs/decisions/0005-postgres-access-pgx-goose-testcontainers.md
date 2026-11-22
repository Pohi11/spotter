# 0005 — Postgres from Go: pgx with hand-written SQL, goose migrations, testcontainers in tests

- **Date:** 2026-11-22
- **Status:** Accepted

## Context

The registry is the first Go service with a database, and every later Go service with state follows its pattern. Three choices are needed: how Go talks to Postgres, how the schema is versioned, and how tests get a real database. The project rule is that database tests run against real Postgres, never a mock. The registry's correctness depends on Postgres features: a partial unique index, row locks, and conditional updates under concurrency.

## Options considered

**Database access**
1. **`database/sql` with a driver.** Standard interface, but loses pgx's Postgres-specific features (typed errors with constraint names, native types) unless unwrapped.
2. **pgx v5 directly, hand-written SQL.** Postgres-native, fast, exposes constraint names on errors (needed to map a unique-index violation to a domain error). SQL is visible and reviewable.
3. **sqlc** (type-safe Go generated from SQL). Removes hand-written scanning code, but adds a code generator and a CI freshness check for about ten queries.
4. **An ORM (GORM, ent).** Hides the SQL that the concurrency guarantees depend on.

**Migrations**
5. **goose.** Plain SQL files, a library API, migrations embeddable in the binary.
6. **golang-migrate.** Similar, separate up/down files, CLI-first.
7. **Atlas.** Declarative schema diffing. More powerful, heavier, and more than a handful of tables needs.

**Test database**
8. **A shared database from Compose or CI service containers,** selected by an environment variable. Fast, but tests silently depend on something already running, and are skipped or fail when it isn't.
9. **testcontainers-go.** Each test binary starts its own Postgres container. `go test ./...` works anywhere Docker runs.

## Decision

Options 2, 5, and 9.

- pgx v5 (`pgxpool`) with SQL written by hand in the adapter.
- goose with migrations embedded via `embed.FS`, applied by the service at startup.
- Tests start one Postgres container per test binary (testcontainers-go) and give each test its own schema (`search_path`), migrated from scratch, so tests can run in parallel without seeing each other's rows.

## Consequences

- **Good:** The SQL the guarantees rely on (`FOR UPDATE`, `WHERE revision = $n`, the partial unique index) is right there in the adapter.
- **Good:** The binary carries its migrations; no separate migration step to forget in a deployment.
- **Good:** `go test ./...` needs only Docker, both locally and in CI.
- **Cost:** Scanning rows by hand is repetitive. Revisit sqlc if the query count grows.
- **Cost:** Running migrations at startup means several replicas may try at once. goose takes a Postgres advisory lock, so only one applies them.
- **Cost:** The first test in each package waits a few seconds for the container.
