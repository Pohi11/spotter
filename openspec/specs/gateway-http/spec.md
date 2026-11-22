# gateway-http Specification

## Purpose
Defines the gateway: the public HTTP/JSON front door that forwards to the registry's gRPC API, how it reports errors, the OpenAPI document it serves, and how the gateway process is configured, checked, and stopped. REST paths are declared in `proto/spotter/v1/registry.proto`.

## Requirements

### Requirement: Registry operations are available over REST
The gateway SHALL expose every `spotter.v1.RegistryService` operation as JSON over HTTP at the method and path declared for it in `registry.proto`, forwarding each request to the registry over gRPC. JSON field names SHALL be the protobuf JSON names (lowerCamelCase).

#### Scenario: Every operation is routed
- **WHEN** a client calls `POST /v1/models`, `GET /v1/models/{name}`, `POST /v1/models/{model_name}/versions`, `GET /v1/models/{model_name}/versions`, `GET /v1/versions/{version_id}`, `POST /v1/versions/{version_id}:transition`, and `GET /v1/versions/{version_id}/history`
- **THEN** each request reaches the matching registry operation and returns its result as JSON

#### Scenario: Register a version over REST
- **WHEN** a client sends `POST /v1/models` for `eurosat-classifier`, then `POST /v1/models/eurosat-classifier/versions` with an artifact URI, checksum, and framework
- **THEN** the second response is 200 with a JSON version whose `version` is 1 and `stage` is `MODEL_VERSION_STAGE_REGISTERED`

#### Scenario: Transition over REST
- **WHEN** a client sends `POST /v1/versions/{id}:transition` with `toStage`, `reason`, and `expectedRevision`
- **THEN** the response is 200 with the version in the new stage at the next revision

### Requirement: Errors are returned as JSON with a matching HTTP status
A failed request SHALL return a JSON body with the gRPC code, a message, and any error details, and an HTTP status derived from the gRPC code: `NOT_FOUND` → 404, `ALREADY_EXISTS` → 409, `INVALID_ARGUMENT` → 400, `FAILED_PRECONDITION` → 400, `ABORTED` → 409, `UNAVAILABLE` → 503.

#### Scenario: Unknown model
- **WHEN** a client sends `GET /v1/models/no-such-model`
- **THEN** the response is 404 with a JSON body whose `code` is 5 (`NOT_FOUND`)

#### Scenario: Invalid field
- **WHEN** a client registers a version with checksum `abc123`
- **THEN** the response is 400 and the JSON details include a field violation for `artifact_sha256`

#### Scenario: Stale revision
- **WHEN** a client transitions a version with an out-of-date `expectedRevision`
- **THEN** the response is 409

#### Scenario: Registry down
- **WHEN** the registry cannot be reached and a client sends any registry request
- **THEN** the response is 503

### Requirement: The OpenAPI document is served
The gateway SHALL serve an OpenAPI document generated from the protobuf definitions at `GET /openapi.json`, describing every REST path above, with path parameters in JSON (lowerCamelCase) form.

#### Scenario: Fetch the document
- **WHEN** a client sends `GET /openapi.json`
- **THEN** the response is 200 JSON whose `paths` include `/v1/models` and `/v1/versions/{versionId}:transition`

### Requirement: The gateway process is configured from the environment and stops gracefully
The gateway SHALL read `REGISTRY_ADDR` (required), `HTTP_ADDR` (default `:8080`), and `SHUTDOWN_TIMEOUT` (default `10s`) from environment variables, failing startup with an error naming any missing or invalid variable. It SHALL answer `GET /livez` with 200 while running and `GET /readyz` with 200 only when the registry's gRPC health check reports `SERVING`, otherwise 503. On SIGINT or SIGTERM it SHALL finish in-flight requests within `SHUTDOWN_TIMEOUT` and exit without error.

#### Scenario: Missing registry address
- **WHEN** the gateway starts without `REGISTRY_ADDR`
- **THEN** startup fails with an error naming `REGISTRY_ADDR`

#### Scenario: Readiness follows the registry
- **WHEN** the registry reports `SERVING`, and later stops answering
- **THEN** `/readyz` returns 200, and later 503, while `/livez` returns 200 throughout

#### Scenario: Stop signal
- **WHEN** the running gateway receives a stop signal
- **THEN** it returns without error within the shutdown timeout
