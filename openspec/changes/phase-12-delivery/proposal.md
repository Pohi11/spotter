# Proposal

## Why

The platform installs on Kubernetes from the chart, but only after building its images locally; nothing publishes them, and there is no way to show the running system to someone without a cluster and a checkout. Delivery needs published images, rebuilt on every change, and a one-command demo that runs on any single machine.

## What Changes

- **Image pipeline:** a GitHub Actions workflow builds the Go, Python, and dashboard images for `linux/amd64` and `linux/arm64` on every pull request, and on pushes to `main` publishes them to GitHub Container Registry tagged with the commit SHA and `main`.
- **Demo deployment:** `deploy/demo/compose.yaml` runs the whole platform (Postgres, NATS, SeaweedFS, registry, gateway, monitor, worker, dashboard) from the published images on one machine. Only the dashboard gets a host port; credentials come from a `demo.env` file that is never committed. A `bootstrap` profile trains a first model version; putting it into production stays a person's decision.
- Decision record 0020.

## Capabilities

### New Capabilities
- `platform-delivery`: how images are built and published, and how the demo runs.

### Modified Capabilities
None.

## Impact

- **New:** `.github/workflows/images.yml`, `deploy/demo/`, `deploy/demo/compose_test.go`, decision record 0020.
- **Out of scope:** choosing and paying for a public host, which needs an account; the demo runs on any machine with Docker.
