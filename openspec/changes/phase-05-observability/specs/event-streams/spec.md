# Spec Delta

## ADDED Requirements

### Requirement: Trace context survives the outbox
When a registry change is written, the W3C trace context active at that moment SHALL be stored with each of its outbox records, and the relay SHALL attach it as the `traceparent` header when it publishes the record, so the published event belongs to the trace of the request that caused it.

#### Scenario: Transition inside a traced request
- **WHEN** a version is transitioned by a gRPC call that carries a `traceparent`
- **THEN** the published `ModelVersionStageChanged` message carries a `traceparent` in the same trace

#### Scenario: Change without a trace
- **WHEN** a change is written with no active trace
- **THEN** its event is published without a `traceparent` header
