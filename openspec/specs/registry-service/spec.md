# registry-service Specification

## Purpose
Defines how the registry process is configured, started, checked, and stopped, so it runs the same way on a laptop and in a Kubernetes cluster, and the dependency rule for its core code.

## Requirements

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

### Requirement: Migrations run at startup
On startup the registry SHALL bring the database schema up to date before serving. Starting against an up-to-date schema SHALL change nothing.

#### Scenario: Empty database
- **WHEN** the registry starts against an empty database
- **THEN** the registry's tables exist before the readiness endpoint reports ready

#### Scenario: Second start
- **WHEN** the registry starts twice against the same database
- **THEN** the second start succeeds and applies no migrations

### Requirement: Liveness and readiness are reported over HTTP
The registry SHALL answer `GET /livez` with 200 while the process is running, and `GET /readyz` with 200 when the database answers a ping within one second, or 503 otherwise.

#### Scenario: Running with a reachable database
- **WHEN** the registry is running and its database is reachable
- **THEN** `/livez` and `/readyz` both return 200

#### Scenario: Database unreachable
- **WHEN** the database does not answer a ping
- **THEN** `/readyz` returns 503 and `/livez` still returns 200

### Requirement: Shutdown is graceful
On SIGINT or SIGTERM the registry SHALL mark itself `NOT_SERVING` in its gRPC health service, stop accepting new HTTP and gRPC connections, let in-flight requests and calls finish within `SHUTDOWN_TIMEOUT`, close its database connections, and exit without error.

#### Scenario: Stop signal while running
- **WHEN** the running registry receives a stop signal
- **THEN** it returns without error within the shutdown timeout
- **AND** neither its HTTP port nor its gRPC port accepts connections

### Requirement: Core code does not depend on infrastructure
The registry's `domain`, `ports`, and `app` packages SHALL NOT import database, network, messaging, or storage packages, and SHALL NOT import the registry's adapters. The build SHALL fail if they do.

#### Scenario: Dependency check
- **WHEN** the dependency check runs over `domain`, `ports`, and `app`
- **THEN** none of them imports `net/...`, `database/...`, `os/exec`, an adapter package, or any third-party module other than `github.com/google/uuid`
