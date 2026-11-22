# Spec Delta

## Purpose

Defines how the gateway exposes the platform's events (inference, registry, and monitor) to HTTP clients such as the dashboard.

## ADDED Requirements

### Requirement: The gateway streams platform events as server-sent events
`GET /v1/events` SHALL respond with `Content-Type: text/event-stream` and send each message from the `INFERENCE`, `REGISTRY`, and `MONITOR` streams as one SSE event whose `data` is a JSON object with `stream`, `subject`, `sequence` (stream sequence), `time` (RFC 3339), `type` (the fully qualified protobuf message name from the `Spotter-Event-Type` header), and `payload` (the message decoded to protobuf JSON). The SSE `id` SHALL be `<stream>:<sequence>`.

#### Scenario: Live completion
- **WHEN** a client is connected and an `InferenceCompleted` event is published on `inference.completed.eurosat-classifier`
- **THEN** the client receives an event with stream `INFERENCE`, type `spotter.v1.InferenceCompleted`, and a payload holding the job id and prediction

#### Scenario: Unknown payload type
- **WHEN** a message has no `Spotter-Event-Type` header or names a type the gateway doesn't know
- **THEN** the client receives the event with its stream, subject, sequence, and time, and a null payload

### Requirement: A new client receives the recent past first
On connect the feed SHALL first send the messages published in the last `since` (a Go duration query parameter, default `10m`, at most `24h`), in stream order per stream, then live messages. An invalid or out-of-range `since` SHALL be answered with 400 before any event is sent.

#### Scenario: Replay then live
- **WHEN** three events were published before a client connects with the default window, and one more is published after
- **THEN** the client receives the three earlier events and then the new one

#### Scenario: Bad window
- **WHEN** a client asks for `since=72h`
- **THEN** the response is 400 and names the `since` parameter

### Requirement: A disconnected client leaves nothing behind
The feed SHALL read the streams with ephemeral ordered consumers that are removed when the client disconnects or the gateway shuts down, and SHALL send an SSE comment at least every 15 seconds so idle connections stay open through proxies.

#### Scenario: Client disconnects
- **WHEN** a connected client closes the connection
- **THEN** the streams have no consumers left from that client

#### Scenario: Idle connection
- **WHEN** no event is published for the keep-alive interval
- **THEN** the client receives a keep-alive comment
