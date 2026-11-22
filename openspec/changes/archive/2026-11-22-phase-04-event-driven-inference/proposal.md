# Proposal

## Why

There is a registered model but no way to use it. Inference has to work the way the rest of the platform will: services that never call each other directly, work that survives a worker crashing mid-job, and events other services can react to (the drift monitor, later). The registry also writes events to its outbox but nothing publishes them, so no other service can learn that a version was promoted.

## What Changes

- Define the JetStream streams and the job-status key-value bucket in code, created idempotently at service startup.
- Publish the registry's outbox to JetStream: each record once per message id, then marked published.
- Add `spotter.v1.InferenceService` to the gateway (gRPC and REST): submit an image for a model and get a job id; look the job up later. The gateway stores the image in the object store, records the job as pending, and publishes an `InferenceRequested` event.
- Turn the Python worker into an inference worker: pull jobs from a durable JetStream consumer, load the model's production version (verifying the artifact's checksum), classify the image, record exactly one result per job, publish an `InferenceCompleted` event, and acknowledge. Transient failures are retried with a delay; jobs that can never succeed are failed, recorded, and sent to a dead-letter subject.
- The worker reloads its model when a version of its model enters or leaves production.
- Add `spotterctl infer <image>` to submit an image and wait for the result.

## Capabilities

### New Capabilities
- `event-streams`: Which streams and buckets exist, how they are created, and how the registry's outbox is published.
- `inference-api`: Submitting inference jobs and reading their status through the gateway.
- `inference-processing`: How the worker processes jobs: model loading, retries, idempotency, dead-lettering, and model reloads.

### Modified Capabilities
- `registry-service`: configuration gains `NATS_URL`; the registry publishes its outbox while running.
- `gateway-http`: configuration gains `NATS_URL`, `GRPC_ADDR`, and object-store settings; readiness also requires a NATS connection.

## Impact

- **Proto:** new `inference.proto` (service, job, prediction, events).
- **Go:** nats.go JetStream API; a shared `internal/natsjs` package; the gateway gains domain, ports, and adapters (NATS, S3, key-value); the registry gains an outbox relay.
- **Python:** the inference worker gains torch (via `spotter-model`), boto3, nats-py JetStream, and grpcio.
- **Tests:** Go tests use an embedded JetStream server; S3 tests use a SeaweedFS container. Python tests use the Compose NATS and SeaweedFS.
- **Out of scope:** batching and optimized runtimes (a later change), tracing across the bus (a later change), autoscaling on queue depth (a later change), streaming results to clients.
