# Spec Delta

## Purpose

Defines the registry's gRPC API: which operations it offers, how registry errors appear as gRPC status codes, and how clients check the server's health and discover its services. Message shapes are defined in `proto/spotter/v1/registry.proto`.

## ADDED Requirements

### Requirement: Registry operations are available over gRPC
The registry SHALL serve `spotter.v1.RegistryService` over gRPC with these operations: register a model, get a model by name, register a version, get a version by id, list a model's versions newest first, transition a version, and get a version's history. Each operation SHALL have the behavior the `model-registry` and `model-lifecycle` capabilities define.

#### Scenario: Register and read back a version
- **WHEN** a client registers model `eurosat-classifier`, registers a version of it, and gets that version by id
- **THEN** the returned version has number 1, stage `MODEL_VERSION_STAGE_REGISTERED`, revision 1, and the artifact and metrics that were sent

#### Scenario: Transition and read history
- **WHEN** a client transitions a version from registered to validated with its current revision, then gets the version's history
- **THEN** the transition returns the version at revision 2 in stage validated
- **AND** the history has one entry from registered to validated with the given reason

#### Scenario: List versions
- **WHEN** a model has three versions and a client lists them
- **THEN** the response holds versions 3, 2, 1 in that order

### Requirement: Registry errors map to gRPC status codes
Every registry error SHALL be returned with exactly one gRPC status code:

| Registry error | gRPC code |
|---|---|
| not found | `NOT_FOUND` |
| already exists | `ALREADY_EXISTS` |
| invalid input | `INVALID_ARGUMENT` |
| illegal transition | `FAILED_PRECONDITION` |
| production occupied | `FAILED_PRECONDITION` |
| stale revision | `ABORTED` |

Any other error SHALL be returned as `INTERNAL` with a generic message that does not expose internal details.

#### Scenario: Unknown version
- **WHEN** a client gets a version by an id that does not exist
- **THEN** the call fails with `NOT_FOUND`

#### Scenario: Duplicate model
- **WHEN** a client registers a model name that already exists
- **THEN** the call fails with `ALREADY_EXISTS`

#### Scenario: Illegal transition
- **WHEN** a client moves a registered version to production
- **THEN** the call fails with `FAILED_PRECONDITION`

#### Scenario: Stale revision
- **WHEN** a client transitions a version with an out-of-date expected revision
- **THEN** the call fails with `ABORTED`

#### Scenario: Unexpected error
- **WHEN** the registry fails for a reason that is not a registry error
- **THEN** the call fails with `INTERNAL` and a message that does not include the underlying error text

### Requirement: Invalid input names the offending field
An `INVALID_ARGUMENT` response SHALL carry a `google.rpc.BadRequest` detail naming each invalid field, using the request's field names. A request with an unknown or unspecified stage, or a malformed id, SHALL be rejected as invalid input.

#### Scenario: Malformed checksum
- **WHEN** a client registers a version with checksum `abc123`
- **THEN** the call fails with `INVALID_ARGUMENT` and a field violation for `artifact_sha256`

#### Scenario: Unspecified target stage
- **WHEN** a client transitions a version to `MODEL_VERSION_STAGE_UNSPECIFIED`
- **THEN** the call fails with `INVALID_ARGUMENT` and a field violation for `to_stage`

#### Scenario: Malformed version id
- **WHEN** a client gets a version with id `not-a-uuid`
- **THEN** the call fails with `INVALID_ARGUMENT` and a field violation for `version_id`

### Requirement: Health and reflection are served over gRPC
The registry's gRPC server SHALL implement the standard `grpc.health.v1.Health` service, reporting `SERVING` for the overall server and for `spotter.v1.RegistryService` while running and `NOT_SERVING` once shutdown begins. It SHALL support gRPC server reflection so tools can discover its services.

#### Scenario: Health check while running
- **WHEN** a client calls `grpc.health.v1.Health/Check` for `spotter.v1.RegistryService`
- **THEN** the status is `SERVING`

#### Scenario: Reflection lists the service
- **WHEN** a client lists services through reflection
- **THEN** the list includes `spotter.v1.RegistryService`
