# Design

## Context

The registry is the first Go service. It sets the layout every later service follows (decision record [0002](../../../../docs/decisions/0002-monorepo-layout-one-go-module.md)) and the testing conventions for Go code ([0001](../../../../docs/decisions/0001-adopt-openspec-with-cross-language-traceability.md)). No API is exposed yet; the next change adds gRPC and REST on top of the use cases defined here.

## Layout

```
services/registry/
  cmd/registry/            main: config, wiring, signals
  internal/
    domain/                Model, ModelVersion, Stage, transition table, errors, events
    ports/                 Repository, EventEncoder, IDGenerator, Clock
    app/                   use cases (Service)
    adapters/
      postgres/            pgx repository, embedded goose migrations
      memory/              in-memory repository for unit tests
      protoevents/         domain events → protobuf outbox records
    registrytest/          contract suite run against every repository adapter
    archtest/              dependency rule check
internal/spectest/         Go spec-to-test helper (shared by all services)
internal/pgtest/           real Postgres for tests (shared by all services)
tools/speccheck/           spec coverage check
```

## Decisions

### D1. The lifecycle is a table in the domain package
`domain.transitions` maps each stage to the stages it may move to. `ModelVersion.Transition` checks the table, requires a non-blank reason, and returns an updated copy with the revision increased by one plus a `StageChange` record. It does no I/O, so the exhaustive test over every (from, to) pair runs in microseconds. `retired → production` exists only so a later change can restore the previous version during a rollback.

### D2. Optimistic concurrency with a revision number
Callers pass the revision they last read. The use case rejects a mismatch early; the repository makes the final check (`UPDATE … WHERE id = $1 AND revision = $2`) so two racing transactions can't both succeed. Under Postgres's default READ COMMITTED isolation, the second `UPDATE` waits for the first to commit, re-reads the row, finds the revision changed, and updates zero rows, which the adapter reports as a conflict. `stage_changes (version_id, revision)` is also unique, as a second line of defense.

### D3. "One production version per model" is a partial unique index
`CREATE UNIQUE INDEX … ON model_versions (model_id) WHERE stage = 'production'`. The adapter maps a violation of that index to `domain.ErrProductionOccupied`. Application code doesn't check first: a check-then-write would race, and the index makes the check unnecessary. The in-memory adapter enforces the same rule so both pass the same contract tests.

### D4. Version numbers are assigned inside the repository transaction
The Postgres adapter locks the model row (`SELECT … FOR UPDATE`), reads `max(version) + 1`, and inserts, so concurrent registrations for one model are serialized and numbering has no gaps. Registrations for different models don't block each other. Because the event payload includes the number, `AddVersion` takes a callback that builds the outbox records once the number is known, inside the same transaction.

### D5. Outbox records are opaque to the repository
The use case turns domain events into `ports.OutboxRecord` values (id, type, aggregate id, payload bytes, time) through the `EventEncoder` port. The protobuf encoder lives in an adapter, so neither `domain` nor `app` imports generated code or the protobuf runtime. The repository stores records without understanding them, which keeps both repository adapters simple. The record id is a UUIDv7 and will become the NATS message id for de-duplication when publishing is added.

### D6. One contract suite, every adapter
`registrytest.Run(t, newRepository)` exercises the spec scenarios through `app.Service` with a given repository. The memory adapter's test and the Postgres adapter's test both call it. Scenarios that only make sense for the database (a direct SQL insert bypassing the application) live in the Postgres adapter's own tests.

### D7. Postgres access and migrations
pgx v5 with hand-written SQL; goose migrations embedded in the binary and applied at startup. Tests run against a real Postgres started by testcontainers, with a fresh schema per test. See decision record [0005](../../../../docs/decisions/0005-postgres-access-pgx-goose-testcontainers.md).

### D8. Spec traceability for Go
Each test calls `spectest.Verifies(t, capability, requirement)`; `tools/speccheck` joins the `go test -json` report with the specs. See decision record [0006](../../../../docs/decisions/0006-go-spec-test-traceability.md).

### D9. The dependency rule is a test
`archtest` runs `go list -deps -json` on `domain`, `ports`, and `app`, and fails on forbidden standard-library imports (`net/...`, `database/...`, `os/exec`), on any module package outside the allowed set, and on any third-party module other than `github.com/google/uuid`. A test needs no extra tool in CI.

### D10. Kubernetes-ready process from the start
Configuration comes only from environment variables (decision record [0004](../../../../docs/decisions/0004-local-dev-topology.md)). `/livez` answers without touching dependencies, so a slow database doesn't get the pod restarted; `/readyz` pings the database, so traffic is withheld until it answers. `main` turns SIGINT/SIGTERM into context cancellation; `run` shuts the HTTP server down with `SHUTDOWN_TIMEOUT` and closes the pool.

## Risks and trade-offs

- **The in-memory adapter can drift from Postgres.** Mitigation: both pass the same contract suite, including the concurrency scenarios.
- **Tests need Docker.** Accepted: the same is true of the services themselves. CI runners have Docker.
- **The rollback edge (`retired → production`) is reachable through the generic transition use case.** Accepted for now; the promotion change replaces direct production transitions with gated promote and rollback use cases.
