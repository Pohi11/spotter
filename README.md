# Spotter

A small, cloud-neutral platform for deploying and monitoring computer-vision models on overhead imagery. Register a model version, serve it behind REST and gRPC, run inference through an event bus, watch live traffic for drift, retrain, shadow-test the new version, and promote it through code-checked gates and a human approval, with rollback.

Status: the model registry's domain, storage, and process are built; its API comes next. The sections below grow as each part is built.

## Model registry

A model version moves through a fixed set of stages. Only these transitions exist; everything else is rejected:

```mermaid
stateDiagram-v2
    [*] --> registered
    registered --> validated
    registered --> rejected
    validated --> shadow
    validated --> rejected
    shadow --> production
    shadow --> rejected
    production --> retired
    production --> rolled_back
    retired --> production: restore during rollback
    rejected --> [*]
    rolled_back --> [*]
```

- **One production version per model,** enforced by a partial unique index in Postgres, not just by application code.
- **Optimistic concurrency:** every transition states the revision it expects. When two race, exactly one wins; the other gets a conflict.
- **Transactional outbox:** every registration and stage change writes a protobuf event ([registry_events.proto](proto/spotter/v1/registry_events.proto)) in the same transaction as the change.
- **Hexagonal and checked:** `domain`, `ports`, and `app` can't import infrastructure; a test fails the build if they do. The Postgres and in-memory repositories pass one shared contract suite, including the concurrency scenarios.
- **Kubernetes-ready process:** config from environment variables, migrations at startup, `/livez` and `/readyz`, graceful shutdown on SIGTERM.

Specs: [model-registry](openspec/specs/model-registry/spec.md), [model-lifecycle](openspec/specs/model-lifecycle/spec.md), [registry-service](openspec/specs/registry-service/spec.md).

## Architecture

- **Go services:** registry (model versions and their lifecycle), gateway (REST and gRPC front door), monitor (drift detection). Each is hexagonal: domain and use-case code depend only on ports, and Postgres, NATS, and S3 sit behind adapters.
- **Python workers:** inference (PyTorch / ONNX Runtime) and trainer.
- **NATS JetStream** carries jobs and events, at least once, with idempotent consumers.
- **PostgreSQL** for the registry and monitoring data. **SeaweedFS** (S3 API) for images and model artifacts.
- **Kubernetes** via Helm; k3d and Tilt locally, kind in CI.

## How it is built

Spec-driven, one change at a time:

- Each change is an [OpenSpec](openspec/) change: proposal, requirements with WHEN/THEN scenarios, design, and tasks, archived once done.
- Decisions that outlive a change get a [decision record](docs/decisions/).
- Every test names the requirement it verifies, in Go, Python, and TypeScript ([0001](docs/decisions/0001-adopt-openspec-with-cross-language-traceability.md)).

## Run it

Requirements: Docker Desktop, Go 1.27+, [uv](https://docs.astral.sh/uv/), and [buf](https://buf.build/) (only to regenerate code).

```bash
cp .env.example .env
docker compose up -d --wait        # Postgres, NATS JetStream, SeaweedFS (S3 on :8333)
uv sync
```

Check that Go and Python reach each other through NATS:

```bash
uv run spotter-inference           # terminal 1: the Python worker
go run ./cmd/spotterctl ping       # terminal 2: reply from inference-worker (python 3.13.x) in 1.6ms
```

Run the registry against the local Postgres (it migrates on start):

```bash
DATABASE_URL='postgres://spotter:spotter-dev@localhost:5432/spotter?sslmode=disable' go run ./services/registry/cmd/registry
curl localhost:8080/readyz
```

## Test

```bash
go test ./...                      # Go; Postgres tests start their own container. CI adds -race (needs cgo)
go run gotest.tools/gotestsum@v1.13.0 --jsonfile go-test.json -- ./... && go run ./tools/speccheck
                                   # every spec requirement has a passing linked test
uv run pytest                      # Python; integration tests need NATS_URL=nats://localhost:4222
uv run ruff check . && uv run ruff format --check .
buf lint && buf generate           # generated code is committed; CI fails if it is stale
```

## Data

EuroSAT (public Sentinel-2 land-use tiles) plus synthetic degradations. No restricted or sensitive data.
