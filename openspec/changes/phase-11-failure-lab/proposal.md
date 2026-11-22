# Proposal

## Why

The event-driven design was chosen for how it behaves when parts fail: jobs wait on the bus instead of being dropped, and at-least-once delivery with idempotent consumers turns retries into exactly one result. That claim has been tested piece by piece, but never measured as a whole system under faults, or against the obvious alternative: the gateway calling the worker directly.

## What Changes

- **A synchronous baseline:** the inference worker can serve `spotter.v1.ClassifierService/Classify` over gRPC, and the gateway exposes `ClassifySync` (`POST /v1/models/{model_name}:classify`), which checks the model and calls the worker directly: no object store, no job store, no bus. It exists for the comparison and is off unless `WORKER_ADDR` is set.
- **`spotterchaos`** (Go): runs closed-loop load in either mode while it injects one fault and heals it: the worker killed and restarted (it owns the worker process), NATS restarted, Postgres paused, or the object store paused. It reports per mode and fault: requests, rejected requests, completed, lost, duplicate completions, latency percentiles, recovery time, and a per-second timeline.
- The worker can run without its shadow model (`SHADOW=off`) so both modes do the same work per job.
- A dated report and decision record 0019 with the conclusion and its numbers.

## Capabilities

### New Capabilities
- `failure-lab`: how the platform's behavior under faults is measured.

### Modified Capabilities
- `inference-api`: adds synchronous classification for comparison.

## Impact

- **Proto:** `ClassifierService` and `ClassifySync` (additive).
- **Gateway:** a new use case, port, and gRPC adapter; unchanged unless `WORKER_ADDR` is set.
- **Worker:** an optional gRPC server (`GRPC_PORT`) and `SHADOW`.
- **New:** `cmd/spotterchaos`, `internal/chaos`, a benchmark report, decision record 0019.
