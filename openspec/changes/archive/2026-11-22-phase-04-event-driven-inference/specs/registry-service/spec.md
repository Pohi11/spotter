# Spec Delta

## MODIFIED Requirements

### Requirement: Configuration comes from environment variables
The registry SHALL read its configuration only from environment variables: `DATABASE_URL` and `NATS_URL` (required), `HTTP_ADDR` (default `:8080`), `GRPC_ADDR` (default `:9090`), and `SHUTDOWN_TIMEOUT` (default `10s`). A missing or invalid value SHALL stop startup with an error that names the variable.

#### Scenario: Missing database URL
- **WHEN** the registry starts without `DATABASE_URL`
- **THEN** startup fails with an error naming `DATABASE_URL`

#### Scenario: Missing NATS URL
- **WHEN** the registry starts without `NATS_URL`
- **THEN** startup fails with an error naming `NATS_URL`

#### Scenario: Invalid shutdown timeout
- **WHEN** the registry starts with `SHUTDOWN_TIMEOUT=soon`
- **THEN** startup fails with an error naming `SHUTDOWN_TIMEOUT`

#### Scenario: gRPC address defaults
- **WHEN** the registry starts without `GRPC_ADDR`
- **THEN** it serves gRPC on port 9090
