# Design

## Context

Every service already behaves the way Kubernetes expects: configuration from environment variables, migrations at startup, `/livez` and `/readyz`, graceful shutdown on SIGTERM (changes `phase-02-registry-api` through `phase-07-drift-monitor`). This change packages them. The packaging choices are recorded in decision record [0014](../../../docs/decisions/0014-helm-umbrella-chart-keda-scaling.md).

## Decisions

### D1. Two images
`deploy/docker/go.Dockerfile` builds all Go binaries (registry, gateway, monitor, `spotterctl`) into one distroless, non-root image; each Deployment picks its command. `deploy/docker/python.Dockerfile` installs the uv workspace (CPU-only PyTorch) into one slim image for the worker and the trainer. Fewer images to build and load into the cluster; each workload still runs a single process.

### D2. Chart layout
`deploy/helm/spotter`: one template per workload plus `_helpers.tpl` for names, labels, and the shared environment (NATS URL, S3 endpoint, registry address). NATS is a dependency on the official chart at 2.15.3 (the version Compose runs), vendored under `charts/` with `Chart.lock`, configured for JetStream with file storage and its monitoring port (needed by KEDA). Release name `spotter` gives service names like `spotter-registry` and `spotter-nats`.

### D3. Credentials
`templates/secret.yaml` generates `postgres-password`, `database-url`, `s3-access-key`, and `s3-secret-key` with `randAlphaNum` on first install and reuses existing values found with `lookup` on upgrade. Workloads use `secretKeyRef` only. `auth.existingSecret` replaces the generated Secret; `externalDatabase.existingSecret` supplies `database-url` when the bundled Postgres is off. SeaweedFS writes its S3 identity from the same Secret at startup (the Compose entrypoint, now in a ConfigMap; the script holds no credentials).

### D4. Probes, resources, shutdown
Go services probe `/livez` (liveness) and `/readyz` (readiness) on their HTTP port; the worker, which serves only `/metrics`, gets TCP probes on that port and a startup probe that allows time to wait for JetStream and load models. Every container has requests and limits. `terminationGracePeriodSeconds` is the service's `SHUTDOWN_TIMEOUT` plus a margin. The worker's `MODEL_THREADS` is derived from its CPU limit (whole cores, at least 1), following the thread-pool finding in change `phase-08-promotion`.

### D5. KEDA
`templates/worker-scaledobject.yaml` (when `worker.autoscaling.enabled`) uses KEDA's `nats-jetstream` scaler against NATS's monitoring endpoint, stream `INFERENCE`, consumer `inference-worker-<model>`, with `lagThreshold`, min and max replicas, polling interval, and cooldown from values. The Deployment omits `replicas` when autoscaling is on so KEDA (through its HPA) owns it. All worker replicas share the one durable pull consumer, so adding replicas adds consumers of the same backlog without duplicating work.

### D6. Smoke test as a Helm test
`templates/tests/smoke.yaml` is a `helm.sh/hook: test` Pod running `spotterctl smoke` from the Go image: wait for the gateway's and registry's `/readyz`, register a uniquely named model through the gateway's REST API and read it back, and ping the worker over NATS. The same command works against a port-forwarded cluster.

### D7. Chart tests in Go
`deploy/helm/chart_test.go` renders the chart with `helm template` (skipped if `helm` isn't on PATH) and checks the rendered objects for each render-time scenario. `TestFreshInstall` and `TestUpgradeKeepsCredentials` run only when `SPOTTER_KUBE_TEST=1`: they run `helm test` and `helm upgrade` against the current cluster. CI's kind job runs them and adds their report to spec coverage.

### D8. Local loop
`deploy/k3d/cluster.yaml` defines a one-node k3d cluster with a local image registry. The `Tiltfile` builds both images, installs KEDA from its chart, installs the Spotter chart with `values-dev.yaml`, and port-forwards the gateway (8080 REST, 9091 gRPC) and NATS (4222). `tilt up` on an empty cluster brings up everything.

## Risks and trade-offs

- **Single-instance Postgres, NATS, and SeaweedFS:** a pod restart is an outage for that dependency (jobs wait in JetStream's file store). Acceptable for development and CI; production should use replicated or managed dependencies.
- **The Python image is large** (CPU PyTorch). Build caching keeps rebuilds to the source layers.
- **KEDA reads lag from NATS's monitoring port,** which must be reachable inside the cluster and is unauthenticated. Fine within the cluster network; a NetworkPolicy should restrict it in a shared cluster.
