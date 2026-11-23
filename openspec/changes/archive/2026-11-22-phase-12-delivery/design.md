# Design

## Context

Images exist as Dockerfiles in `deploy/docker/`, built locally or in the kind job. The chart takes image repositories and tags as values (change `phase-09-kubernetes`). The choices are recorded in decision record [0020](../../../../docs/decisions/0020-ghcr-images-and-a-compose-demo.md).

## Decisions

### D1. One workflow, a matrix of three images
`.github/workflows/images.yml` builds each image with Buildx for both platforms (QEMU for arm64), with the GitHub Actions cache per image. Pull requests build without pushing, so a broken Dockerfile fails before merge; pushes to `main` log in with the workflow's token and push. The dashboard's build context gets the benchmark reports from `sync-content.mjs` first, as in the kind job.

### D2. The demo is Compose, not Kubernetes
A demo host is one small VM. Compose runs the same images with health checks and restart policies and no control plane to operate. The chart stays the production path; the demo is for showing the system.

### D3. Secrets and exposure
Compose's `${VAR:?message}` makes rendering fail without each secret, so a demo can't start with a default password. Every service but the dashboard stays on the Compose network; the dashboard reaches the gateway by service name, as in the chart.

### D4. A model needs a person
A fresh install has no model. The `bootstrap` profile runs `spotter-train` (default 3 epochs) with `SHADOW_AFTER_VALIDATION=true`, which registers version 1, validates it against the floor, and puts it in shadow, as retraining does. Approving and promoting it is left to the person running the demo, in the dashboard, as the promotion design requires; with no production version the approval is the only gate that can fail (decision record 0015).

### D5. Tests
`deploy/demo/compose_test.go` renders the file with `docker compose config` (skipped when Docker is missing) and checks the services, ports, and the missing-secret failure. The image workflow is checked by a test that parses it. The running demo is verified once from locally built images and recorded in the change's results.
