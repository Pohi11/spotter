# Proposal

## Why

Every later part of Spotter (serving, drift monitoring, shadow testing, promotion, rollback) needs one answer to "which model versions exist, where are their artifacts, and which one is live?" That answer has to be strict. A boolean `is_active` can't tell a version that was never validated apart from one that was rolled back, and it can't stop a rejected version from going live. The version lifecycle has to be an explicit state machine, enforced in plain code and in the database, before anything is allowed to act on it.

The registry is also the first service built in the hexagonal layout every Go service follows. Its domain and use-case code must not depend on Postgres, so the boundary is checked by a test from the first commit.

## What Changes

- Add the registry's domain: models, model versions with a verifiable artifact (S3 URI, SHA-256, framework, metrics), and the version lifecycle as a table of allowed stage transitions.
- Add use cases: register a model, register a version (numbered sequentially per model), transition a version's stage with a reason and an expected revision, and read versions and their history.
- Enforce "at most one production version per model" in the database and report it as a domain error.
- Detect concurrent transitions of the same version with optimistic concurrency (a revision number) and reject the stale one.
- Write an event for every registration and stage change to a transactional outbox, in the same transaction as the change. Event payloads are protobuf messages defined in `proto/`. Publishing them comes in a later change.
- Add two repository adapters, PostgreSQL and in-memory, that pass one shared contract test suite.
- Add the registry process: configuration from environment variables, migrations at startup, liveness and readiness endpoints, and graceful shutdown.
- Add the Go spec-to-test convention and a coverage check that fails the build when a requirement has no passing test.
- Add a test that fails the build if domain, port, or use-case code imports infrastructure.

## Capabilities

### New Capabilities
- `model-registry`: Models and model versions, the rules for what a valid version is, version numbering, and lookups.
- `model-lifecycle`: The stages a model version moves through, the allowed transitions, history, the one-production-version rule, concurrent updates, and outbox events.
- `registry-service`: How the registry process is configured, started, checked, and stopped, and the dependency rule for its core code.

### Modified Capabilities

None. This is the first change with spec-level behavior.

## Impact

- **Go dependencies:** pgx v5, goose v3, google/uuid, testcontainers-go (tests only), gotestsum (CI only).
- **Proto:** new `spotter/v1/registry_events.proto`.
- **Database:** tables for models, model versions, stage changes, and the outbox.
- **CI:** the Go job runs tests through gotestsum to produce a JSON report and then runs the spec coverage check. Tests need Docker for Postgres.
- **Out of scope:** the gRPC/REST API (next change), publishing outbox events to NATS, promotion gates, human approval, and the atomic promote/rollback swap (later changes).
