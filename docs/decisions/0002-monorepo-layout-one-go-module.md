# 0002 — Monorepo layout: one Go module, one uv workspace, committed generated code

- **Date:** 2026-11-22
- **Status:** Accepted

## Context

Spotter has three Go services (registry, gateway, monitor), two Python workers (inference, trainer), a TypeScript dashboard, and protobuf contracts used by both Go and Python. Everything lives in one repository.

Three questions have to be answered before the first line of service code:

1. How many Go modules?
2. How do the two Python projects share dependencies and generated code?
3. Is code generated from `.proto` files committed, or generated during every build?

The failure this layout must avoid: a build that works on a laptop because a local workspace file resolves imports from disk, then fails (or silently uses an older published version) inside a Docker build that only sees part of the repository.

## Options considered

**Go modules**

1. **One module per service, joined by `go.work` for local development.** Each service owns its dependency versions. Cross-module imports (the shared generated code) need `go.work` or `replace` directives locally, and every Dockerfile has to reproduce that setup or the build resolves the shared module from the network.
2. **One module at the repository root.** One `go.mod`, one `go.sum`. Services are package trees inside it. Generated code is just another package. Every service shares the same dependency versions.

**Python projects**

3. **Two independent uv projects,** each with its own lock file and its own copy of the generated code.
4. **One uv workspace** at the repository root with the two workers and the generated protobuf package as members, sharing one lock file.

**Generated code**

5. **Generate during each build.** The repository holds only `.proto` files. Every build environment needs `buf` and its plugins.
6. **Commit generated code,** and have CI regenerate it and fail if the result differs from what is committed.

## Decision

Options 2, 4, and 6.

```
go.mod                      one Go module: github.com/Pohi11/spotter
services/
  registry/
    cmd/registry/           main package: wiring only
    internal/
      domain/               types and rules, no I/O imports
      app/                  use cases, depend only on ports
      ports/                interfaces
      adapters/             postgres, nats, s3, grpc, http
  gateway/                  same shape
  monitor/                  same shape
proto/spotter/v1/           .proto files (source of truth for shapes)
gen/go/spotter/v1/          generated Go (committed)
gen/python/                 generated Python package (committed, uv workspace member)
pyproject.toml              uv workspace root
workers/inference/          uv workspace member
workers/trainer/            uv workspace member
dashboard/                  Next.js (pnpm)
deploy/                     compose, helm, tilt
openspec/  docs/
```

- **Every Docker build uses the repository root as its context,** with a `.dockerignore` that limits what is sent. A Dockerfile can then copy exactly the paths a service needs, and there is no workspace file to forget.
- **Each service's code lives under its own `internal/`.** The Go compiler forbids importing another service's `internal/` packages, so services can only depend on each other through `gen/` contracts, never through each other's domain types.

## Consequences

- **Good:** No `go.work` and no `replace` directives, so a laptop build and a Docker build resolve imports the same way.
- **Good:** Service boundaries are enforced by the compiler (`internal/`), and the hexagonal layer rules can be checked inside each service.
- **Good:** Generated code is visible in code review, and Docker builds and the IDE need no code generator. CI's regenerate-and-diff check catches a forgotten `buf generate`.
- **Cost:** All Go services share dependency versions. Upgrading a library for one upgrades it for all. At three services owned by one team that's acceptable; at many teams it wouldn't be.
- **Cost:** Every service image build can be invalidated by a change anywhere in `go.mod`. Build caching and narrow `COPY` lines limit the damage.
- **Cost:** Committed generated code makes diffs larger when `.proto` files change.
- **Revisit when:** services need independent release cycles or dependency versions, or a second team owns one of them.
