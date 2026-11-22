# observability Specification

## Purpose
Defines how Spotter's services trace requests across processes and the message bus, what spans may contain, how telemetry leaves a process, and which metrics each service exposes.

## Requirements

### Requirement: Every request continues or starts a trace
Each service SHALL continue the W3C trace context of an incoming gRPC call, HTTP request, or NATS message, or start a new trace if there is none, and pass the context on with every gRPC call it makes and every NATS message it publishes.

#### Scenario: gRPC call continues the caller's trace
- **WHEN** a client calls the gateway with a `traceparent` header
- **THEN** the gateway's server span belongs to the caller's trace

#### Scenario: Gateway to worker over NATS
- **WHEN** an image is submitted through the gateway
- **THEN** the `InferenceRequested` message carries a `traceparent` header in the same trace as the gateway's server span

#### Scenario: Worker continues the trace and passes it on
- **WHEN** the worker processes a message that carries a `traceparent` header
- **THEN** its processing span belongs to that trace
- **AND** the `InferenceCompleted` message it publishes carries a `traceparent` in the same trace

### Requirement: Spans never contain image bytes or secrets
Span names and attributes SHALL NOT contain image data, credentials, or connection strings. Spans MAY contain ids, model names, versions, stages, outcomes, sizes, and durations.

#### Scenario: Submitting an image
- **WHEN** an image is submitted and processed
- **THEN** no recorded span attribute contains the image bytes or any configured credential

### Requirement: Telemetry export is configured by environment
Each service SHALL export spans over OTLP when `OTEL_EXPORTER_OTLP_ENDPOINT` is set, using `OTEL_SERVICE_NAME` (defaulting to the service's name), and SHALL run normally with tracing as a no-op when it is not set.

#### Scenario: No collector configured
- **WHEN** a service starts without `OTEL_EXPORTER_OTLP_ENDPOINT`
- **THEN** it serves requests normally and still propagates trace context

### Requirement: Services expose Prometheus metrics
The registry and the gateway SHALL serve `GET /metrics` in the Prometheus text format with request counts and duration histograms for their gRPC and HTTP endpoints. The worker SHALL serve `/metrics` with jobs processed by outcome, a histogram of time spent in the model, and the number of messages pending for its consumer.

#### Scenario: Gateway metrics after requests
- **WHEN** a client has made gRPC calls to the gateway and then fetches `/metrics`
- **THEN** the response includes a gRPC server duration histogram with a non-zero count

#### Scenario: Worker outcomes
- **WHEN** the worker completes one job and dead-letters another
- **THEN** its `/metrics` shows one job with outcome `succeeded` and one with outcome `failed`
