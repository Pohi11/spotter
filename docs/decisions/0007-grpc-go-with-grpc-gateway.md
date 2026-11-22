# 0007 — gRPC with grpc-go; REST and OpenAPI from the same proto via grpc-gateway

- **Date:** 2026-11-22
- **Status:** Accepted

## Context

The registry needs an API that other services call (typed, versioned, efficient) and that people and simple tools can call with `curl`. Message shapes already live in `proto/` (decision record [0002](0002-monorepo-layout-one-go-module.md)). The API, its REST form, and its documentation should come from one definition so they can't drift apart, and breaking changes should be caught by `buf breaking`.

## Options considered

1. **grpc-go, plus grpc-gateway for REST.** The reference gRPC implementation for Go. grpc-gateway generates a reverse proxy from `google.api.http` annotations in the proto, and `protoc-gen-openapiv2` generates OpenAPI from the same file. Widely used, well understood. Two servers in the path for REST callers (gateway → registry).
2. **connect-go.** One `net/http` handler serves gRPC, gRPC-Web, and the Connect protocol (JSON over HTTP POST). Simpler server code, but the Connect protocol isn't REST-shaped (no resource paths or GET); REST paths need a further layer (vanguard-go), which is younger.
3. **Hand-written REST handlers next to gRPC.** Full control over the HTTP API, but two definitions of every operation to keep in sync by hand.
4. **GraphQL.** Strong for flexible reads across related data; the registry's operations are few, command-shaped, and called by services, which suits RPC better.

## Decision

Option 1.

- The registry serves `spotter.v1.RegistryService` with grpc-go, plus the standard health service and server reflection.
- REST paths are `google.api.http` options on each RPC. The gateway service hosts the grpc-gateway mux and forwards to the registry over gRPC.
- The OpenAPI v2 document is generated from the proto and served by the gateway.
- Generated code for all of it is committed and checked for staleness in CI.

## Consequences

- **Good:** One file defines the gRPC API, the REST paths, and the docs; `buf breaking` protects all of them.
- **Good:** Internal callers use gRPC directly; only external callers pay for the extra hop.
- **Good:** The most common Go setup, so it's easy for others to work on.
- **Cost:** More generated code and plugins to pin (gRPC, gateway, OpenAPI, Python gRPC).
- **Cost:** OpenAPI v2 rather than v3.
- **Cost:** REST URLs follow Google's API style (`:transition` custom methods), which some HTTP clients find unusual.
