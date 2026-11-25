# Spec Delta

## ADDED Requirements

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
