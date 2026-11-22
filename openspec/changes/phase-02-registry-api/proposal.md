# Proposal

## Why

The registry's use cases exist but nothing outside the process can call them. The trainer needs to register versions, operators need to move versions through stages, and later the inference worker needs to find the production version. Other services need a typed, versioned contract (gRPC); people and simple tools need plain HTTP and JSON. Both have to come from one definition so they can't drift apart.

## What Changes

- Add `spotter.v1.RegistryService` in `proto/spotter/v1/registry.proto`: register and get models; register, get, list, and transition versions; read a version's history. REST paths are declared on each RPC in the same file.
- Serve the API over gRPC from the registry, with the standard gRPC health service and server reflection.
- Map every registry error to one gRPC status code, in one place. Validation errors carry the name of the offending field as a structured error detail.
- Add the gateway service, the public HTTP front door. It translates REST/JSON requests into registry gRPC calls, returns errors as JSON with the matching HTTP status, and serves an OpenAPI document generated from the protobuf definitions.
- Add `GRPC_ADDR` to the registry's configuration, and stop the gRPC server gracefully on shutdown.

## Capabilities

### New Capabilities
- `registry-api`: The registry's operations over gRPC, the error-to-status mapping, health checking, and reflection.
- `gateway-http`: The REST/JSON front door for registry operations, its error responses, the OpenAPI document, and the gateway process (configuration, health, shutdown).

### Modified Capabilities
- `registry-service`: configuration gains `GRPC_ADDR`; graceful shutdown covers the gRPC server.

## Impact

- **Proto:** new `registry.proto` with `google.api.http` annotations; `buf.yaml` gains the googleapis dependency. Generated code adds gRPC stubs (Go, Python), grpc-gateway handlers (Go), and an OpenAPI v2 document.
- **Go dependencies:** grpc-go, grpc-gateway v2, genproto (googleapis).
- **Python dependencies:** `googleapis-common-protos` and `grpcio` for the generated package.
- **New service:** `services/gateway`.
- **Out of scope:** authentication and authorization, inference endpoints (a later change), streaming APIs, and pagination (model and version counts are small).
