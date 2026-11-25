# Design

## Context

`deploy/demo/compose.yaml` runs the platform from the GHCR images and publishes only the dashboard (decision record 0020). The choice of host, access control, and hostname is recorded in decision record [0023](../../../docs/decisions/0023-public-demo-host-on-a-vps-behind-caddy.md).

## Decisions

### D1. An override, not a second demo file
`deploy/vps/compose.yaml` is applied after the demo file with `-f`. It adds `caddy` and resets the dashboard's ports with `!reset []`, so the demo file stays the one definition of the platform. Paths in an override resolve from the first file's directory, so the Caddyfile is mounted as `../vps/Caddyfile`.

### D2. Caddy does TLS and the password
Caddy obtains and renews the certificate on its own (HTTP-01 on port 80, kept in the `caddydata` volume) and redirects HTTP to HTTPS. `basic_auth` runs before `reverse_proxy` for every path, including the dashboard's `/api` proxy and its event stream. `flush_interval -1` keeps the server-sent event feed unbuffered. Docker publishes ports past the host firewall, so the protection that matters is which ports are published: only Caddy's 80 and 443.

### D3. Secrets stay on the host
`init-env.sh` runs on the host, generates the database password, object-store keys, and site password with `openssl rand`, hashes the site password with `caddy hash-password`, and writes `vps.env` (mode 600). The plaintext password is kept only in the deploying user's home directory on the host. The hash is single-quoted in `vps.env` so Compose does not read its `$` signs as variables. An existing `vps.env` is never overwritten, so a redeploy keeps the database's password.

### D4. Starting order
`deploy.sh` waits for the platform (`up --wait`), then starts the retrainer, which subscribes to the monitor's stream and exits if the stream does not exist yet. A fresh host still needs the `bootstrap` run to train version 1, and a person to promote it.

### D5. A 4 GB host
The platform's idle footprint fits in 4 GB; training (bootstrap or retraining) adds a PyTorch process. `setup-server.sh` adds a 2 GiB swap file when the host has none, so a training peak slows down instead of triggering the OOM killer.

### D6. Tests
`deploy/vps/compose_test.go` renders the merged file (only Caddy publishes ports; a missing hash fails), adapts the Caddyfile with Caddy itself (authentication before the proxy, one site, the dashboard as upstream), and, when pointed at a deployed host, checks 401 without credentials, 200 with them, and the HTTP-to-HTTPS redirect.
