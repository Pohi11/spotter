# 0014 — One Helm chart for the platform; KEDA scales workers on JetStream lag

- **Date:** 2026-11-22
- **Status:** Accepted

## Context

The services have run on a laptop next to Docker Compose dependencies (decision record [0004](0004-local-dev-topology.md)). The target is Kubernetes, including clusters that are disconnected or not run by a cloud vendor, so the deployment must be code, install into an empty cluster, and avoid vendor services. Inference load arrives in bursts, and a worker can only process one job at a time. How many workers are needed depends on the backlog, not on CPU.

## Options considered

Packaging:

1. **Plain manifests with Kustomize overlays.** No templating language, but no dependency management and no install/upgrade/test lifecycle.
2. **One Helm umbrella chart** with the platform's workloads as templates and NATS as a dependency on its official chart.
3. **An operator.** The most control, but far more code than this platform's lifecycle needs.

Postgres:

4. **A managed database.** Not available in a disconnected cluster, and ties the chart to a vendor.
5. **An operator (CloudNativePG).** Failover, backups, and upgrades, at the cost of CRDs and an operator to install first.
6. **A single-instance StatefulSet in the chart**, with a switch to use an external database instead.

Scaling the worker:

7. **HorizontalPodAutoscaler on CPU.** The worker's CPU doesn't reflect the backlog: a burst queues jobs long before CPU rises.
8. **KEDA on JetStream consumer lag.** Scales on the number of jobs waiting for the worker's durable consumer.

## Decision

Options 2, 6, and 8.

- The chart `deploy/helm/spotter` declares the registry, gateway, monitor, and worker (Deployments), Postgres and SeaweedFS (single-instance StatefulSets), an optional trainer Job, and NATS with JetStream through the official chart, vendored in the repository so installs don't need the network.
- Credentials (database password, S3 keys) go in a Secret that the chart generates at first install and keeps on upgrade. An existing Secret can be supplied instead. Nothing secret lives in values files.
- KEDA scales the worker Deployment between configured bounds on the lag of `inference-worker-<model>` on the `INFERENCE` stream.
- Local development uses k3d and Tilt; CI installs the chart into kind and runs a smoke test. Images are built locally and loaded into the cluster; publishing images is left to CI/CD.

## Consequences

- **Good:** One `helm install` brings up the platform from an empty cluster (plus KEDA when autoscaling is on), and `helm test` checks it.
- **Good:** Nothing vendor-specific: the same chart runs on k3d, kind, or any conformant cluster.
- **Cost:** The bundled Postgres has no failover or backups. A real deployment should point the chart at an operator-managed or external database (`postgres.enabled=false`).
- **Cost:** KEDA is a cluster-wide prerequisite for autoscaling. Without it, `worker.autoscaling.enabled=false` falls back to a fixed replica count.
