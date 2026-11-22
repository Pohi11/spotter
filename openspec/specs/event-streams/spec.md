# event-streams Specification

## Purpose
Defines the JetStream streams and key-value bucket Spotter uses, how they come into existence, and how the registry's outbox reaches them.

## Requirements

### Requirement: Streams and buckets are defined in code
Services SHALL create or update, at startup, the streams and bucket they rely on: stream `INFERENCE` for subjects `inference.>`, stream `REGISTRY` for subjects `registry.events.>`, and key-value bucket `inference_jobs`. Both streams SHALL de-duplicate messages by message id over a window of at least two minutes. Creating them when they already exist with the same configuration SHALL succeed and change nothing.

#### Scenario: First start
- **WHEN** a service starts against a NATS server with no streams
- **THEN** streams `INFERENCE` and `REGISTRY` and bucket `inference_jobs` exist with the configured subjects

#### Scenario: Repeated start
- **WHEN** the setup runs twice
- **THEN** the second run succeeds and the streams keep their messages

#### Scenario: Duplicate message id
- **WHEN** two messages with the same message id are published to `INFERENCE` within the de-duplication window
- **THEN** the stream stores one message

### Requirement: The registry publishes its outbox
While running, the registry SHALL publish every unpublished outbox record to `registry.events.<event>` (for example `registry.events.model_version_stage_changed`), with the record id as the message id and the event type in a `Spotter-Event-Type` header, and mark the record published only after the server acknowledges it. Records SHALL be published in the order they were written.

#### Scenario: A transition reaches the stream
- **WHEN** a version is transitioned while the registry is running
- **THEN** a `spotter.v1.ModelVersionStageChanged` message for it appears on `registry.events.model_version_stage_changed`
- **AND** its outbox record is marked published

#### Scenario: Publish fails
- **WHEN** NATS is unavailable while records are pending
- **THEN** the records stay unpublished and are published once NATS is available again

#### Scenario: Republished after a crash
- **WHEN** a record was published but the registry stopped before marking it published, and the registry publishes it again
- **THEN** the stream still holds one message for that record

### Requirement: Trace context survives the outbox
When a registry change is written, the W3C trace context active at that moment SHALL be stored with each of its outbox records, and the relay SHALL attach it as the `traceparent` header when it publishes the record, so the published event belongs to the trace of the request that caused it.

#### Scenario: Transition inside a traced request
- **WHEN** a version is transitioned by a gRPC call that carries a `traceparent`
- **THEN** the published `ModelVersionStageChanged` message carries a `traceparent` in the same trace

#### Scenario: Change without a trace
- **WHEN** a change is written with no active trace
- **THEN** its event is published without a `traceparent` header
