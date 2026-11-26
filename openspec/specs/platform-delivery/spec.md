# platform-delivery Specification

## Purpose
Defines how Spotter's images are built and published, and how the single-machine demo runs.

## Requirements

### Requirement: Images are built on every change and published from main
The image workflow SHALL build the Go, Python, and dashboard images for `linux/amd64` and `linux/arm64` on every pull request and every push to `main`, only after every other CI job has passed, and SHALL push them to `ghcr.io/<owner>/spotter-go`, `spotter-python`, and `spotter-dashboard` only on pushes to `main`, tagged with the full commit SHA and `main`. The image workflow SHALL not run except from CI.

#### Scenario: Workflow definition
- **WHEN** the workflow file is read
- **THEN** it builds all three images for both platforms, logs in and pushes only when asked to publish, and tags with the commit SHA and `main`

#### Scenario: Gated on CI
- **WHEN** the CI workflow is read
- **THEN** its `images` job calls the image workflow, needs every other CI job with no condition that could run it after a failure, and asks to publish only for push events, and the image workflow has no trigger other than being called

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

### Requirement: The demo can retrain on drift without a terminal
`deploy/demo/compose.yaml` SHALL define a `retrainer` service, only in the `retrain` profile, running `spotter-retrain --watch` from the Python image with the registry, NATS, and object-store settings and the bootstrap's dataset volume. It SHALL publish no port and SHALL not be part of the demo without the profile.

#### Scenario: Rendered with the retrain profile
- **WHEN** the file is rendered with `--profile retrain`
- **THEN** it contains a `retrainer` service running `spotter-retrain --watch` that mounts the same dataset volume as `bootstrap` and publishes no port

#### Scenario: Rendered without it
- **WHEN** the file is rendered without the profile
- **THEN** it has no `retrainer` service

### Requirement: Only the proxy is reachable on the public demo host
`deploy/vps/compose.yaml`, applied over `deploy/demo/compose.yaml`, SHALL add a Caddy service that is the only service publishing host ports: 80 and 443. The dashboard SHALL publish no port. The file SHALL refuse to render without `SITE_ADDRESS`, `BASIC_AUTH_USER`, or `BASIC_AUTH_HASH`.

#### Scenario: Rendered for the host
- **WHEN** both files are rendered with every setting
- **THEN** only Caddy publishes ports, exactly 80/tcp, 443/tcp, and 443/udp, and it mounts `deploy/vps/Caddyfile`

#### Scenario: Missing password hash
- **WHEN** they are rendered without `BASIC_AUTH_HASH`
- **THEN** rendering fails naming `BASIC_AUTH_HASH`

### Requirement: The public demo host requires TLS and a password
Caddy SHALL serve `SITE_ADDRESS` over HTTPS, redirect HTTP to HTTPS, and require HTTP basic authentication with `BASIC_AUTH_USER` and the bcrypt hash `BASIC_AUTH_HASH` on every path before forwarding to the dashboard.

#### Scenario: Caddy configuration
- **WHEN** Caddy adapts the Caddyfile
- **THEN** there is one HTTPS site for `SITE_ADDRESS` whose handlers authenticate with HTTP basic auth before proxying to `dashboard:3000`

#### Scenario: Running host
- **WHEN** a deployed host is requested
- **THEN** a request without credentials gets 401 on the page and on the API proxy, a request with them gets 200, and plain HTTP redirects permanently to HTTPS
