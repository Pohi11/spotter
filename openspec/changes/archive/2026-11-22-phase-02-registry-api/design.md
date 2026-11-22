# Design

## Context

Phase 1 built the registry's use cases behind ports. This change adds the first driving adapter (gRPC) and the gateway service. The choice of RPC stack is recorded in decision record [0007](../../../../docs/decisions/0007-grpc-go-with-grpc-gateway.md).

## Layout

```
proto/spotter/v1/registry.proto                    service, messages, google.api.http paths
services/registry/internal/adapters/grpcapi/       RegistryService server, error mapping, conversions
services/registry/cmd/registry/                    now serves gRPC (+ health, reflection) and HTTP health
services/gateway/internal/restapi/                 grpc-gateway mux, error handler, OpenAPI handler
services/gateway/cmd/gateway/                      config, registry connection, health, shutdown
internal/svc/                                      shared process helpers: env parsing, HTTP/gRPC serve and shutdown
gen/openapi/spotter/v1/registry.swagger.json        generated OpenAPI v2 (embedded in the gateway)
```

## Decisions

### D1. One proto file defines gRPC, REST, and OpenAPI
Each RPC carries a `google.api.http` option. `buf generate` produces the Go server interface, the grpc-gateway reverse proxy, the OpenAPI document, and Python stubs. Changing a path or field happens in one place, and `buf breaking` guards all three.

### D2. Error mapping lives in one function
`grpcapi.toStatus(err)` checks registry errors with `errors.Is`/`errors.As` and returns the status from the spec's table. `ValidationError` becomes `INVALID_ARGUMENT` with a `google.rpc.BadRequest` detail. Domain field names already match the proto field names (`artifact_sha256`, `reason`), so no translation table is needed. Anything unrecognised is logged with its full text and returned as `INTERNAL` with a fixed message, so database errors never reach clients.

### D3. Request parsing errors are validation errors
Ids are parsed with `uuid.Parse` and stages mapped from the proto enum in the adapter; a failure produces the same `BadRequest` detail as a domain validation error. The domain never sees a malformed id or an unspecified stage.

### D4. Stage values travel as the proto enum
The API reuses `spotter.v1.ModelVersionStage` from the events file, so events and API responses use the same values. JSON renders enum names (`MODEL_VERSION_STAGE_SHADOW`).

### D5. The gateway is a stateless proxy for now
It holds a gRPC client connection to `REGISTRY_ADDR` and hands it to the grpc-gateway mux. It owns no data yet; inference submission arrives in a later change and will give it its own ports and adapters. HTTP status mapping uses grpc-gateway's standard table (`runtime.HTTPStatusFromCode`), which already matches the spec.

### D6. Readiness checks the next hop
The gateway's `/readyz` calls the registry's `grpc.health.v1.Health/Check` with a one-second timeout. The registry's own readiness checks its database. Kubernetes then withholds traffic from a gateway whose registry is down, instead of letting requests fail with 503.

### D7. Shutdown order
On a stop signal the registry sets its health status to `NOT_SERVING` first (load balancers stop routing), then runs `http.Server.Shutdown` and `grpc.Server.GracefulStop` in parallel, bounded by `SHUTDOWN_TIMEOUT`; if the deadline passes, `grpc.Server.Stop` forces remaining calls closed. The pool closes last.

### D8. Tests run in-process
gRPC adapter tests use `bufconn` (an in-memory listener) with the in-memory repository, so they run in milliseconds. Gateway tests point the grpc-gateway mux at a bufconn registry server, exercising real HTTP→gRPC translation without opening ports. Process tests (`cmd/...`) use real listeners on `127.0.0.1:0` and, for the registry, a real Postgres schema.

## Risks and trade-offs

- **No authentication.** Accepted for a local demo; any deployment beyond a laptop needs it before exposing the gateway.
- **grpc-gateway emits OpenAPI v2, not v3.** Accepted; v2 is enough for documentation and client generation.
- **A second hop adds latency** for REST callers. Accepted; internal services call the registry over gRPC directly.
