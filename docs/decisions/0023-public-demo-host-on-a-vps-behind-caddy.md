# 0023 — Host the public demo on a VPS with Compose, behind Caddy with TLS and a password

- **Date:** 2026-11-25
- **Status:** Accepted

## Context

Decision record 0020 made the demo a Compose file that runs the whole platform on one machine, and left open where a public copy runs. A public copy has to serve the model, not only the dashboard: the guided demo sends real tiles through the inference API, and the monitor, retrainer, and promotion flow need the full platform behind it. The demo has no authentication of its own, and anyone who reaches it can send traffic, approve, promote, and roll back (decision record 0016).

## Options considered

For the host:

1. **Hugging Face Docker Space:** Docker Spaces on the free CPU tier now need a paid subscription. A Space runs one container, so every process would run under a supervisor, the second way to run everything that 0020 rejected. Its disk resets on restart unless persistent storage is bought, and it sleeps when idle.
2. **A static or serverless host for the dashboard (Vercel, a static Space):** serves the dashboard and its recorded replay, but not the model, the event bus, or the services.
3. **One small VPS running the existing Compose file:** the same images and separation as the demo, persistent volumes, no sleep, at a lower monthly price than option 1. A 4 GB, 2 vCPU machine fits the platform; a swap file covers training peaks. The host is OVHcloud's VPS-1; the cheaper Hetzner plans could not be ordered at the time.

For access:

1. **Fully open:** anyone who finds the address can promote, roll back, or flood the model.
2. **Public reading, a password for actions:** needs changes in the dashboard and the proxy to tell reading from acting, and the guided demo itself sends traffic.
3. **One shared password on the whole site:** no code changes; viewers get the password with the link.

For the hostname: a real domain, or an `sslip.io` name (`<ip-with-dashes>.sslip.io`), which resolves to the address it contains and can get a public certificate with no DNS setup.

## Decision

A VPS runs `deploy/demo/compose.yaml` with an override, `deploy/vps/compose.yaml`, that adds Caddy and removes the dashboard's host port. Caddy is the only service with host ports (80 and 443). It obtains a Let's Encrypt certificate for `SITE_ADDRESS`, redirects HTTP to HTTPS, requires one shared basic-auth login on every path, and forwards to the dashboard on the Compose network. The hostname starts as the `sslip.io` name of the host's address. Secrets and the password are generated on the host (`init-env.sh`) and never leave it; `vps.env` holds the password's bcrypt hash. Scripts in `deploy/vps/` prepare the host and ship the files, so the host is rebuilt from the repository, not by hand.

## Consequences

- **Good:** The public copy is the real platform: inference, events, drift, retraining, and promotion all run, from the same published images as every other deployment.
- **Good:** Nothing but Caddy is reachable from outside, and nothing changes in the dashboard or services.
- **Cost:** One password for everyone: no per-person accounts, and the approval name in the dashboard is whatever a viewer types. Changing the password means regenerating the hash and restarting Caddy.
- **Cost:** One machine, no redundancy; a VPS restart is downtime, and its backups are the provider's.
- **Cost:** The `sslip.io` name depends on a third-party DNS service; moving to a real domain changes only `SITE_ADDRESS`.
