# Proposal

## Why

An inference request now crosses four processes (client, gateway, JetStream, worker) and a registry event crosses a database transaction and a relay before a worker reacts to it. When something is slow or fails, logs from each process can't be lined up. Operators need one trace per request across every hop, and rate, error, and duration metrics for each service, before later changes add more services and failure modes.

## What Changes

- Add OpenTelemetry tracing to the registry, the gateway, and the worker. Each continues an incoming trace or starts one, and exports spans over OTLP when `OTEL_EXPORTER_OTLP_ENDPOINT` is set (no-op otherwise).
- Propagate W3C trace context across gRPC and HTTP (standard instrumentation) and across NATS (in message headers): gateway → `inference.requested` → worker → `inference.completed`.
- Carry trace context through the registry's outbox: the context active when a change is written is stored with its event and attached when the relay publishes it.
- Expose Prometheus metrics at `GET /metrics` on every service: request rate, errors, and duration for gRPC and HTTP; the worker's job outcomes, inference time, and its consumer's pending messages.
- Add Jaeger (trace UI) and Prometheus (metrics) to the local Compose stack, configured in the repository.

## Capabilities

### New Capabilities
- `observability`: Tracing across processes and the bus, what spans may contain, how telemetry is exported, and the metrics each service exposes.

### Modified Capabilities
- `event-streams`: outbox records carry the trace context of the change that wrote them, and the relay attaches it when publishing.

## Impact

- **Go:** OpenTelemetry SDK, OTLP exporter, otelgrpc and otelhttp instrumentation, the Prometheus exporter; a shared `internal/telemetry` package; a migration adding `trace_parent` to the outbox.
- **Python:** OpenTelemetry SDK and OTLP exporter, `prometheus-client`.
- **Compose:** Jaeger and Prometheus services and a scrape configuration.
- **Out of scope:** dashboards (a later change), log correlation, alerting, sampling configuration beyond the default.
