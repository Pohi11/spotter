# Tasks

## 1. Foundations

- [ ] 1.1 Write decision record 0010 (OpenTelemetry traces, Prometheus metrics).
- [ ] 1.2 Add `internal/telemetry` and `internal/natstrace`. Verify with their tests.

## 2. Go services

- [ ] 2.1 Instrument the gateway: gRPC and HTTP, NATS publish, `/metrics`. Verify the gateway scenarios with an in-memory span recorder.
- [ ] 2.2 Instrument the registry: gRPC, `/metrics`, outbox `trace_parent` (migration, both adapters, publisher header). Verify the outbox scenarios.

## 3. Worker

- [ ] 3.1 Add tracing (extract, spans, inject) and Prometheus metrics. Verify the worker scenarios against Compose NATS and SeaweedFS.

## 4. Local backends and a real trace

- [ ] 4.1 Add Jaeger and Prometheus to Compose. Run the stack, classify images, and confirm one trace in Jaeger spans gateway and worker. Record the result.
