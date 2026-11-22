# Spec Delta

## MODIFIED Requirements

### Requirement: The gateway process is configured from the environment and stops gracefully
The gateway SHALL read `REGISTRY_ADDR`, `NATS_URL`, `S3_ENDPOINT`, `S3_ACCESS_KEY`, and `S3_SECRET_KEY` (all required), and `HTTP_ADDR` (default `:8080`), `GRPC_ADDR` (default `:9091`), `S3_REGION` (default `us-east-1`), `IMAGES_BUCKET` (default `images`), and `SHUTDOWN_TIMEOUT` (default `10s`) from environment variables, failing startup with an error naming any missing or invalid variable. It SHALL answer `GET /livez` with 200 while running and `GET /readyz` with 200 only when the registry's gRPC health check reports `SERVING` and the NATS connection is open, otherwise 503. On SIGINT or SIGTERM it SHALL finish in-flight requests within `SHUTDOWN_TIMEOUT` and exit without error.

#### Scenario: Missing registry address
- **WHEN** the gateway starts without `REGISTRY_ADDR`
- **THEN** startup fails with an error naming `REGISTRY_ADDR`

#### Scenario: Missing NATS URL
- **WHEN** the gateway starts without `NATS_URL`
- **THEN** startup fails with an error naming `NATS_URL`

#### Scenario: Readiness follows the registry
- **WHEN** the registry reports `SERVING`, and later stops answering
- **THEN** `/readyz` returns 200, and later 503, while `/livez` returns 200 throughout

#### Scenario: Readiness follows NATS
- **WHEN** the NATS connection is closed
- **THEN** `/readyz` returns 503

#### Scenario: Stop signal
- **WHEN** the running gateway receives a stop signal
- **THEN** it returns without error within the shutdown timeout
