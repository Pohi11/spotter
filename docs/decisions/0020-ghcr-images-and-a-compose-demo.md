# 0020 — Publish multi-arch images to GHCR; run the demo with Compose on one machine

- **Date:** 2026-11-22
- **Status:** Accepted

## Context

The images were built only on a developer's machine and inside the CI kind job. Installing the chart anywhere else, or showing the platform to someone, meant building them first. A demo host is likely a small VM, possibly an ARM one (several free tiers are ARM), and the project has no cloud account committed to it.

## Options considered

For images:

1. **Docker Hub:** a separate account and token, pull-rate limits for anonymous users.
2. **GitHub Container Registry:** the repository's own token can push, images link to the repository, public packages pull without limits.
3. **A cloud registry** (ECR, Artifact Registry): ties delivery to one vendor, against the cloud-neutral goal.

For the demo:

1. **k3s on a VM with the Helm chart:** the production path, but a control plane to keep alive on a small machine, and KEDA, the NATS chart, and StatefulSets for a demo.
2. **One container with every process inside:** smallest footprint (as a Hugging Face Space would need), but a second way to run everything, with a process supervisor and no health checks per service.
3. **Docker Compose on one VM from the published images:** the same images and the same separation as the chart, with health checks and restarts, and nothing to operate beyond Docker.

## Decision

GHCR, built for `linux/amd64` and `linux/arm64` on every pull request and published from `main` with SHA and `main` tags. The demo is Compose (`deploy/demo/`), with only the dashboard exposed and credentials from an uncommitted env file.

## Consequences

- **Good:** Any machine with Docker, x86 or ARM, runs the platform with two commands; the chart can install from the published images by setting `images.*.repository`.
- **Good:** Pull requests build all three images, so a broken Dockerfile fails before merge.
- **Cost:** Building the Python image for arm64 under emulation is slow; the cache keeps rebuilds to changed layers.
- **Cost:** The demo has no TLS or authentication of its own; it is meant to sit behind a reverse proxy or be shared for a limited time, and anyone who can reach it can approve and promote (decision record 0016).
