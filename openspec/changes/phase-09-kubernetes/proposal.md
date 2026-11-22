# Proposal

## Why

The platform runs only as processes on a laptop next to Docker Compose. The target is Kubernetes, including disconnected clusters, so it must install from code into an empty cluster, keep credentials out of the repository, stop cleanly, and add inference workers when jobs back up rather than when CPU rises.

## What Changes

- Container images: one Go image (registry, gateway, monitor, `spotterctl`) and one Python image (inference worker, trainer).
- A Helm umbrella chart for the whole platform: registry, gateway, monitor, and worker Deployments; Postgres and SeaweedFS StatefulSets; NATS with JetStream from the official chart (vendored); an optional trainer Job; generated credentials in a Secret; probes, resources, and graceful-shutdown settings on every workload.
- KEDA scales the worker on JetStream consumer lag.
- `helm test` runs a smoke test (`spotterctl smoke`): the gateway and registry answer, a model can be registered and read back through the REST API, and the worker answers a NATS ping.
- Local development: a k3d cluster definition and a Tiltfile that builds the images and installs KEDA and the chart.
- CI: install the chart into kind and run the smoke test.

## Capabilities

### New Capabilities
- `platform-deployment`: How the platform is packaged and installed on Kubernetes, how it handles credentials, health, shutdown, and scaling, and how an install is verified.

### Modified Capabilities
None.

## Impact

- **New:** `deploy/docker/`, `deploy/helm/spotter/`, `deploy/k3d/`, `Tiltfile`, `spotterctl smoke`, a CI job.
- **Unchanged:** service behaviour. The services already read configuration from the environment, migrate at startup, expose `/livez` and `/readyz`, and shut down on SIGTERM.
- **Out of scope:** publishing images to a registry, multi-node Postgres or NATS, ingress and TLS, running Jaeger and Prometheus in the cluster.
