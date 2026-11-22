# Tasks

## 1. Decisions

- [ ] 1.1 Decision records 0016 (dashboard and event feed) and 0017 (TypeScript test traceability); speccheck reads requirement tags in JUnit test names (`speccheck_test.go`).

## 2. Platform

- [ ] 2.1 `DriftEvaluated` in `monitor.proto`; the monitor publishes it at every evaluation (`app` tests with fakes).
- [ ] 2.2 Gateway event feed: `domain.Event`, `ports.EventSource`, `adapters/natsevents`, `GET /v1/events` handler (tests against embedded JetStream: live, unknown type, replay, bad window, disconnect, keep-alive).

## 3. Dashboard

- [ ] 3.1 Next.js app skeleton with Poppins, tokens, layout, navigation; proxy with allowlist (Vitest).
- [ ] 3.2 Sample export script (`spotter-demo-samples`, pytest) and worked-example recorder.
- [ ] 3.3 Home page: introduction, contact sheet, worked example, classify panel, how an image moves, guarantees, skills.
- [ ] 3.4 Live page and event summaries (Vitest).
- [ ] 3.5 Model page: versions, gates, approve, promote, rollback (Vitest for gate and rollback logic).
- [ ] 3.6 Drift page and PSI series (Vitest).
- [ ] 3.7 Results page from `benchmarks/*.json` (Vitest for the loader).
- [ ] 3.8 Architecture page with diagrams.
- [ ] 3.9 Playwright tests against the stub gateway, including phone width and keyboard focus.

## 4. Packaging and CI

- [ ] 4.1 Dashboard Dockerfile, Compose service, chart Deployment and Service (chart test).
- [ ] 4.2 CI: dashboard lint, type check, Vitest, Playwright; reports joined into spec coverage.
- [ ] 4.3 Real run: the dashboard against the live stack (classify, live events, approve, promote, rollback, drift).
