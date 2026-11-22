# Proposal

## Why

Every part of the platform works, but seeing it work takes a terminal, `curl`, Jaeger, and Prometheus side by side. An operator deciding whether to trust a deployment, and anyone evaluating the design, needs one place that shows the events moving through the bus, each model version's stage and gates, the drift statistics, and the measured results, and that explains how the parts fit together.

## What Changes

- **Gateway event feed:** `GET /v1/events` streams platform events (inference, registry, and monitor) to HTTP clients as server-sent events, decoded from their protobuf payloads to JSON. A client gets the recent past first and then live events.
- **Drift evaluations are published:** the monitor publishes a `DriftEvaluated` event on `monitor.drift.evaluated.<model>` at every evaluation, so PSI can be charted over time without scraping Prometheus. `DriftDetected` and `DriftResolved` are unchanged.
- **Dashboard** (`dashboard/`, Next.js, TypeScript, Tailwind): a home page that explains the platform with one recorded image's path and a panel to classify a sample tile, a live events page, a model page (versions, stages, gates, approval, promotion, rollback), a drift page, a results page built from the dated benchmark reports, and an architecture page with diagrams. The browser reaches the gateway only through an allowlisted proxy.
- **Demo samples:** a script exports EuroSAT holdout tiles and their synthetic degradations (the same functions the drift experiment uses) for the panel.
- The Helm chart gains a dashboard Deployment and Service (locally, `pnpm dev` runs it against the services on the host); CI lints, type-checks, and tests the dashboard and runs its browser tests against a recorded gateway.

## Capabilities

### New Capabilities
- `event-feed`: how the gateway exposes the platform's events to HTTP clients.
- `dashboard`: what the operator dashboard shows and lets a person do, and how it reaches the platform.

### Modified Capabilities
- `drift-monitoring`: every evaluation is published as `DriftEvaluated`; state-change events are unchanged.

## Impact

- **Proto:** `DriftEvaluated` added to `monitor.proto` (additive; `buf breaking` passes).
- **Gateway:** a new route (with a `streams` filter), port, and JetStream adapter; existing routes unchanged.
- **Monitor:** publishes one more event type.
- **New:** `dashboard/`, a dashboard image, a chart Deployment and Service, CI jobs, decision records 0016 (dashboard and event feed) and 0017 (TypeScript test traceability).
- **Out of scope:** authentication (the dashboard targets a local or demo deployment; an approval records the name a person types), and the build walkthrough pages (change `phase-10b-build-walkthrough`).
