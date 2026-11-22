# 0016 — A Next.js dashboard that reads the platform through the gateway, with an SSE event feed

- **Date:** 2026-11-22
- **Status:** Accepted

## Context

The platform needs an operator view: live events, each version's stage and gates, the promotion and rollback actions, drift over time, and the measured results. Its data sits in the registry (Postgres), in JetStream (events and job state), and in Prometheus. The gateway is the only public entry point, and every service behind it is internal. Browsers can't speak NATS's protocol or gRPC without extra infrastructure.

## Options considered

1. **The dashboard connects to NATS and Postgres directly.** Fewest hops, but it would need database and bus credentials, duplicate the registry's rules, and decode protobuf in TypeScript.
2. **A separate backend-for-frontend service.** Clean separation, but one more Go service to build, deploy, and secure, with nothing to do that the gateway can't.
3. **The dashboard reads through the gateway's REST API, and the gateway adds an event feed.** Registry reads and actions already exist there. Events need one new route: server-sent events (one-way, plain HTTP, reconnect built into browsers) rather than WebSockets (two-way, which the feed doesn't need) or polling (latency and wasted requests).

For the dashboard itself: Next.js with TypeScript and Tailwind CSS, as in earlier work; server components render registry data without client-side fetching, and a route handler can proxy the browser's requests and stream the event feed.

## Decision

Option 3. The gateway serves `GET /v1/events`, reading the three streams with ephemeral ordered consumers and decoding each payload by its `Spotter-Event-Type` header. The dashboard's server reaches the gateway at `GATEWAY_URL`; the browser reaches it only through `/api/`, which forwards an allowlist of routes. The monitor publishes `DriftEvaluated` at every evaluation so the drift chart reads the bus like everything else, rather than querying Prometheus.

## Consequences

- **Good:** One entry point and one API contract for people, scripts, and the dashboard; the dashboard holds no credentials.
- **Good:** The live view shows exactly what is on the bus, decoded with the same protobuf schemas the services use.
- **Cost:** The dashboard has no authentication. It suits a laptop or a demo cluster; a shared deployment needs an identity provider in front of it.
- **Cost:** Each connected viewer holds three ordered consumers on the server. A larger audience would need a fan-out in the gateway.
