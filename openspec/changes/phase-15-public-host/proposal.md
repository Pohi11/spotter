# Proposal

## Why

The demo runs on any machine with Docker (change `phase-12-delivery`), but there is no public copy to show, and the demo has no TLS or authentication of its own. A public copy has to serve the model, so it needs the whole platform, not only the dashboard.

## What Changes

- **Public host:** `deploy/vps/compose.yaml` layers Caddy over `deploy/demo/compose.yaml`. Caddy is the only service with host ports, serves the site over HTTPS with a certificate it obtains itself, requires one shared password on every path, and forwards to the dashboard. The dashboard loses its host port.
- **Host as code:** `setup-server.sh` installs Docker and adds swap on a fresh Ubuntu host, `init-env.sh` writes the host's secrets and password hash on the host, and `deploy.sh` ships the files and starts the platform and the retrainer.
- Decision record 0023.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `platform-delivery`: adds the public demo host.

## Impact

- **New:** `deploy/vps/`, decision record 0023.
- **Unchanged:** the demo Compose file, the images, the dashboard, and the services.
