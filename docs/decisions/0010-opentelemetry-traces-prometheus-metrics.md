# 0010 — OpenTelemetry for traces, Prometheus format for metrics

- **Date:** 2026-11-22
- **Status:** Accepted

## Context

Requests now cross several processes, two languages, and a message bus. Operators need to follow one request end to end and to watch each service's rate, errors, and duration. Whatever is chosen must work identically in Go and Python, be vendor-neutral (the platform is cloud-neutral, decision record [0003](0003-object-store-seaweedfs.md)), and carry context across NATS, which has no built-in tracing.

## Options considered

1. **OpenTelemetry traces exported over OTLP; metrics in Prometheus format.** OpenTelemetry is the vendor-neutral standard with SDKs for both languages and ready instrumentation for gRPC and HTTP; OTLP is accepted by Jaeger, Tempo, and commercial backends. Prometheus is the de facto metrics format for Kubernetes.
2. **Vendor agents** (Datadog, New Relic). Polished, but ties the platform to a vendor and a network connection, which disconnected deployments may not have.
3. **Logs only, with correlation ids.** Simple, but no timing breakdown across hops, and correlation has to be threaded by hand anyway.

## Decision

Option 1.

- Traces: OpenTelemetry SDKs, W3C TraceContext propagation over gRPC, HTTP, and NATS headers, OTLP export when `OTEL_EXPORTER_OTLP_ENDPOINT` is set.
- Metrics: Prometheus text format at `/metrics` (via the OpenTelemetry Prometheus exporter in Go and `prometheus_client` in Python).
- Locally: Jaeger for traces and Prometheus for metrics, both in Compose.

## Consequences

- **Good:** One trace follows a request across languages and the bus; any OTLP-capable backend can replace Jaeger without code changes.
- **Good:** Without a collector configured, services still propagate context, so a partially instrumented chain doesn't break traces.
- **Cost:** NATS propagation is hand-written (a small header carrier) and must be used by every publisher and consumer.
- **Cost:** Two metrics libraries across the two languages.
