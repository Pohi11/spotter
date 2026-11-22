# Tasks

## 1. Conventions

- [ ] 1.1 Write decision records 0005 (Postgres access) and 0006 (Go spec traceability). Verify both files exist and the design links to them.
- [ ] 1.2 Add `internal/spectest` (`Verifies`) and `tools/speccheck`. Verify with `go test ./internal/spectest ./tools/speccheck`.

## 2. Domain

- [ ] 2.1 Add stages, the transition table, `ModelVersion.Transition`, validation, errors, and events in `services/registry/internal/domain`. Verify with `domain` tests: every (from, to) pair, blank reason, every validation rule.

## 3. Use cases and the in-memory adapter

- [ ] 3.1 Add `ports`, `app.Service`, the protobuf event encoder, and `registry_events.proto`. Verify `buf lint` and `buf generate`.
- [ ] 3.2 Add the in-memory repository and the contract suite `registrytest.Run`. Verify with `go test ./services/registry/internal/adapters/memory`.

## 4. Postgres adapter

- [ ] 4.1 Add `internal/pgtest`, the goose migration, and the Postgres repository. Verify with `go test ./services/registry/internal/adapters/postgres`, which runs the contract suite and the direct-insert scenario.

## 5. Dependency rule

- [ ] 5.1 Add `archtest`. Verify it passes, and that adding an import of `net/http` to `domain` makes it fail.

## 6. Registry process

- [ ] 6.1 Add `cmd/registry`: config, migrations, `/livez`, `/readyz`, graceful shutdown. Verify with `cmd/registry` tests.

## 7. CI and coverage

- [ ] 7.1 Run Go tests through gotestsum with a JSON report and run `speccheck` in CI. Verify that every requirement in this change has a passing linked test.
