# Tasks

## 1. Contract

- [x] 1.1 Write decision record 0007 (grpc-go with grpc-gateway). Verify the design links to it.
- [x] 1.2 Add `registry.proto` with `google.api.http` paths, the googleapis dependency, and the gRPC, gateway, OpenAPI, and Python gRPC plugins (pinned). Verify `buf lint` and `buf generate`.

## 2. Registry gRPC adapter

- [x] 2.1 Add `grpcapi`: the server, conversions, and `toStatus`. Verify with `grpcapi` tests over bufconn covering every `registry-api` scenario.

## 3. Shared process helpers and the registry process

- [x] 3.1 Add `internal/svc` (env parsing, serve-and-shutdown for HTTP and gRPC). Verify with its tests.
- [x] 3.2 Serve gRPC, health, and reflection from `cmd/registry`; add `GRPC_ADDR`. Verify with `cmd/registry` tests, including shutdown closing both ports.

## 4. Gateway

- [x] 4.1 Add `services/gateway/internal/restapi` and `cmd/gateway`. Verify with tests covering every `gateway-http` scenario.

## 5. Check

- [x] 5.1 Run both services against Compose Postgres and exercise them with `curl` (REST) and a gRPC client. Verify speccheck reports every requirement covered.
