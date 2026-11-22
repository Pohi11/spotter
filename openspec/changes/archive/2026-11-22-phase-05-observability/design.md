# Design

## Context

Requests cross the gateway, JetStream, and the worker (change `phase-04-event-driven-inference`); registry events cross the outbox and the relay. Tracing choices are recorded in decision record [0010](../../../../docs/decisions/0010-opentelemetry-traces-prometheus-metrics.md).

## Decisions

### D1. One telemetry package per language
`internal/telemetry.Setup(ctx, service)` (Go) and `spotter_inference.telemetry.setup()` (Python) install the W3C TraceContext + Baggage propagator always, and an OTLP/gRPC batch exporter only when `OTEL_EXPORTER_OTLP_ENDPOINT` is set. Without it the tracer provider samples nothing but propagation still works, so a service in the middle of a traced flow passes context on.

### D2. Standard instrumentation where it exists
gRPC servers and clients use `otelgrpc` stats handlers; the gateway's REST mux is wrapped in `otelhttp`. These produce the server spans and the `rpc.server.call.duration` / `http.server.request.duration` histograms.

### D3. NATS carries context in headers
`internal/natstrace` adapts `nats.Header` to OpenTelemetry's `TextMapCarrier`. Publishers inject into the message header; the worker extracts from the delivered message's headers and starts a `CONSUMER` span `process inference.requested`, with child spans for downloading the image, classifying, recording the result, and publishing (a `PRODUCER` span that injects into the completion's headers).

### D4. The outbox stores `traceparent`
A migration adds `outbox.trace_parent`. `ports.OutboxRecord` gains `TraceParent string`. The Postgres and memory adapters fill it from the context when inserting (adapters may import OpenTelemetry; domain and app still cannot). The NATS publisher sets the header from the stored value. A later relay pass therefore publishes the event inside the trace of the request that wrote it, linking "operator promoted version 2" to "worker reloaded".

### D5. Metrics: OpenTelemetry in Go, Prometheus client in Python
Go uses the OpenTelemetry metrics SDK with the Prometheus exporter, so instrumentation metrics appear on the existing HTTP listener at `/metrics`. The worker uses `prometheus_client` directly on its own port (`METRICS_PORT`, default 9464): `spotter_inference_jobs_total{outcome}`, `spotter_inference_model_seconds`, and `spotter_inference_consumer_pending` (from the consumer's info, refreshed each fetch loop).

### D6. What spans may hold
Attributes are limited to ids, names, versions, stages, outcomes, sizes, and durations (`spotter.job_id`, `spotter.model`, `spotter.model_version`, `messaging.message.delivery_count`, `spotter.image_bytes`). A test records every span from a submission through processing and asserts the image bytes and the configured S3 secret appear in none of them.

### D7. Local backends as code
Compose gains `jaeger` (all-in-one v2, OTLP on 4317, UI on 16686) and `prometheus` (scraping the services on the host via `host.docker.internal`), with `deploy/compose/prometheus.yml` in the repository.

## Risks and trade-offs

- **Two metrics libraries.** Accepted: each is the idiomatic choice in its language; Prometheus is the common format.
- **Header size:** `traceparent` adds about 60 bytes per message. Negligible.
- **Default sampling is "always on" when exporting.** Fine locally; production would sample.
