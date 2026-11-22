# 0004 — Local development: Compose for dependencies, k3d and Tilt for the cluster, kind in CI

- **Date:** 2026-11-22
- **Status:** Accepted

## Context

Spotter needs three runtime dependencies (PostgreSQL, NATS JetStream, SeaweedFS), five of its own processes (three Go services, two Python workers), and a dashboard. It is deployed to Kubernetes with Helm. Development happens on Windows with Docker Desktop, where file-change events crossing into the Linux VM are slow or unreliable, and memory is limited by Docker Desktop's VM.

Two needs pull in different directions:

- **A fast edit-run loop** for Go and Python code, measured in seconds.
- **Early, frequent proof that the system runs on Kubernetes,** so cluster concerns (configuration, health probes, graceful shutdown, image size) aren't discovered at the end.

## Options considered

1. **Everything in Docker Compose until the end, Kubernetes added last.** Fastest early on. Cluster problems surface late, when they are most expensive.
2. **Everything in a local Kubernetes cluster from day one, through Tilt.** Closest to production, but slow while services are still changing shape daily, and heavy on memory.
3. **Two modes, plus a third in CI.** Compose runs only the dependencies while services run on the host. A local k3d cluster driven by Tilt runs the whole platform from Helm. CI installs the same Helm chart into a kind cluster with no Tilt involved.

For the local cluster tool:

| Tool | Runs | Notes |
|---|---|---|
| kind | upstream Kubernetes in Docker | reference implementation; standard for CI |
| k3d | k3s in Docker | starts in seconds, light on memory, built-in local registry; k3s is common on edge hardware |
| minikube | Kubernetes in a VM or container | most options, slowest, most moving parts |

## Decision

Option 3.

- **Inner loop:** `docker compose up -d` starts Postgres, NATS (JetStream enabled), and SeaweedFS (single-process mode). Go services run with `go run`, Python workers with `uv run`, all on the host.
- **Cluster loop:** `tilt up` against a k3d cluster with a local image registry, deploying the Helm umbrella chart. Used from the first runnable service onward, not saved for the end.
- **CI:** a kind cluster, `helm install` of the same chart, and an end-to-end smoke test. Tilt never runs in CI, so nothing it patches during development can hide a broken chart.
- **Kubernetes-ready from Phase 1,** whichever loop is in use: every service reads configuration only from environment variables, exposes liveness and readiness endpoints, and shuts down cleanly on SIGTERM.
- **Configuration differs only by environment variables.** For example, `NATS_URL` is `nats://localhost:4222` on the host and `nats://nats:4222` inside the cluster.

## Consequences

- **Good:** Seconds-long edit-run loop for daily work, without giving up regular runs on a real cluster.
- **Good:** The chart is proven on two distributions (k3s locally, upstream Kubernetes in CI), which is evidence it isn't tied to one environment.
- **Good:** Kubernetes requirements (environment configuration, probes, SIGTERM handling) are designed in from the start instead of bolted on.
- **Cost:** Three ways to run the system, and three sets of docs to keep current.
- **Cost:** k3s ships its own ingress controller and storage class. The chart must not rely on them without noticing, or it will fail on other clusters; the kind run in CI is the check.
- **Cost:** The full platform in k3d competes for Docker Desktop's memory. Worker replicas and resource requests are kept small in local values.
