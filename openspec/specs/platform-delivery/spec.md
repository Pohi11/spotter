# platform-delivery Specification

## Purpose
Defines how Spotter's images are built and published, and how the single-machine demo runs.

## Requirements

### Requirement: Images are built on every change and published from main
The image workflow SHALL build the Go, Python, and dashboard images for `linux/amd64` and `linux/arm64` on every pull request and every push to `main`, and SHALL push them to `ghcr.io/<owner>/spotter-go`, `spotter-python`, and `spotter-dashboard` only on pushes to `main`, tagged with the full commit SHA and `main`.

#### Scenario: Workflow definition
- **WHEN** the workflow file is read
- **THEN** it builds all three images for both platforms, logs in and pushes only for push events, and tags with the commit SHA and `main`

### Requirement: The demo runs the platform on one machine
`deploy/demo/compose.yaml` SHALL define Postgres, NATS with JetStream, SeaweedFS, the registry, gateway, monitor, inference worker, and dashboard from the images named by `IMAGE_PREFIX` and `IMAGE_TAG` (default `ghcr.io/pohi11` and `main`). Only the dashboard SHALL publish a host port. The database password and object-store keys SHALL come from the environment, and the file SHALL refuse to render without them.

#### Scenario: Rendered with secrets
- **WHEN** the file is rendered with the three secrets set
- **THEN** it contains the eight services, and only the dashboard publishes a port

#### Scenario: Missing secret
- **WHEN** it is rendered without `POSTGRES_PASSWORD`
- **THEN** rendering fails naming `POSTGRES_PASSWORD`

#### Scenario: Running demo
- **WHEN** the demo is started from locally built images
- **THEN** every service reports healthy or running, and the dashboard and its proxy to the gateway answer on the published port
