# Spec Delta

## MODIFIED Requirements

### Requirement: Streams and buckets are defined in code
Services SHALL create or update, at startup, the streams and bucket they rely on: stream `INFERENCE` for subjects `inference.>`, stream `REGISTRY` for subjects `registry.events.>`, stream `MONITOR` for subjects `monitor.>`, and key-value bucket `inference_jobs`. Every stream SHALL de-duplicate messages by message id over a window of at least two minutes. Creating them when they already exist with the same configuration SHALL succeed and change nothing.

#### Scenario: First start
- **WHEN** a service starts against a NATS server with no streams
- **THEN** streams `INFERENCE`, `REGISTRY`, and `MONITOR` and bucket `inference_jobs` exist with the configured subjects

#### Scenario: Repeated start
- **WHEN** the setup runs twice
- **THEN** the second run succeeds and the streams keep their messages

#### Scenario: Duplicate message id
- **WHEN** two messages with the same message id are published to `INFERENCE` within the de-duplication window
- **THEN** the stream stores one message
