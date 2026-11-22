# Tasks

## 1. Contracts and streams

- [x] 1.1 Write decision record 0009 (at-least-once inference with KV job state).
- [x] 1.2 Add `inference.proto` and regenerate. Verify `buf lint` and `buf generate`.
- [x] 1.3 Add `internal/natsjs` (streams, bucket) and `internal/s3test`. Verify the `event-streams` stream scenarios against an embedded JetStream server.

## 2. Registry outbox relay

- [x] 2.1 Add `Repository.MarkPublished` (both adapters, contract tests), `ports.EventPublisher`, `RelayOutbox`, and the NATS publisher. Verify the relay scenarios.
- [x] 2.2 Run the relay in `cmd/registry`; add `NATS_URL`. Verify with `cmd/registry` tests.

## 3. Gateway inference API

- [x] 3.1 Add the gateway's domain, ports, app, adapters (S3, NATS, KV), and the `InferenceService` server with REST. Verify every `inference-api` scenario.
- [x] 3.2 Extend `cmd/gateway` config and readiness. Verify the modified `gateway-http` scenarios.

## 4. Inference worker

- [x] 4.1 Add model loading with checksum verification and production reloads.
- [x] 4.2 Add job processing: idempotent result, ack/nak/term, dead-letter. Verify every `inference-processing` scenario against Compose NATS and SeaweedFS.

## 5. CLI and end-to-end

- [x] 5.1 Add `spotterctl infer`. Run registry, gateway, and worker locally with the trained model promoted to production; classify real EuroSAT images. Record the results.
