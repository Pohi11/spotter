# Spec Delta

## MODIFIED Requirements

### Requirement: Configuration comes from environment variables
The registry SHALL read its configuration only from environment variables: `DATABASE_URL` (required), `HTTP_ADDR` (default `:8080`), `GRPC_ADDR` (default `:9090`), and `SHUTDOWN_TIMEOUT` (default `10s`). A missing or invalid value SHALL stop startup with an error that names the variable.

#### Scenario: Missing database URL
- **WHEN** the registry starts without `DATABASE_URL`
- **THEN** startup fails with an error naming `DATABASE_URL`

#### Scenario: Invalid shutdown timeout
- **WHEN** the registry starts with `SHUTDOWN_TIMEOUT=soon`
- **THEN** startup fails with an error naming `SHUTDOWN_TIMEOUT`

#### Scenario: gRPC address defaults
- **WHEN** the registry starts without `GRPC_ADDR`
- **THEN** it serves gRPC on port 9090

### Requirement: Shutdown is graceful
On SIGINT or SIGTERM the registry SHALL mark itself `NOT_SERVING` in its gRPC health service, stop accepting new HTTP and gRPC connections, let in-flight requests and calls finish within `SHUTDOWN_TIMEOUT`, close its database connections, and exit without error.

#### Scenario: Stop signal while running
- **WHEN** the running registry receives a stop signal
- **THEN** it returns without error within the shutdown timeout
- **AND** neither its HTTP port nor its gRPC port accepts connections
