# Tasks

## 1. Decisions

- [x] 1.1 Decision records 0016 (dashboard and event feed) and 0017 (TypeScript test traceability); speccheck reads requirement tags in JUnit test names (`speccheck_test.go`).

## 2. Platform

- [x] 2.1 `DriftEvaluated` in `monitor.proto`; the monitor publishes it at every evaluation (`app` tests with fakes).
- [x] 2.2 Gateway event feed: `domain.Event`, `ports.EventSource`, `adapters/natsevents`, `GET /v1/events` handler (tests against embedded JetStream: live, unknown type, replay, bad window, disconnect, keep-alive).

## 3. Dashboard

- [x] 3.1 Next.js app skeleton with Poppins, tokens, layout, navigation; proxy with allowlist (Vitest).
- [x] 3.2 Sample export script (`spotter-demo-samples`, pytest) and worked-example recorder.
- [x] 3.3 Home page: introduction, contact sheet, worked example, classify panel, how an image moves, guarantees, skills.
- [x] 3.4 Live page and event summaries (Vitest).
- [x] 3.5 Model page: versions, gates, approve, promote, rollback (Vitest for gate and rollback logic).
- [x] 3.6 Drift page and PSI series (Vitest).
- [x] 3.7 Results page from `benchmarks/*.json` (Vitest for the loader).
- [x] 3.8 Architecture page with diagrams.
- [x] 3.9 Playwright tests against the stub gateway, including phone width and keyboard focus.

## 4. Packaging and CI

- [x] 4.1 Dashboard Dockerfile, chart Deployment and Service (chart test).
- [x] 4.2 CI: dashboard lint, type check, Vitest, Playwright; reports joined into spec coverage.
- [x] 4.3 Real run: the dashboard against the live stack (classify, live events, drift, a retrained version in shadow with its gates). Approving and promoting is left to a person; the browser tests cover that flow against the recorded gateway.
