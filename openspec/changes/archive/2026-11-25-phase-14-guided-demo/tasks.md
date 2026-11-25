# Tasks

## 1. Decision

- [x] 1.1 Decision record 0022.

## 2. Drift arithmetic in events

- [x] 2.1 `FeatureDrift.bin_edges`, `reference_shares`, `window_shares`; regenerate; `buf breaking` passes.
- [x] 2.2 `domain.Compare` and PSI from it; results and the adapter carry the bins (domain test: bins reproduce the PSI; adapter test: published with the evaluation).

## 3. Scenario tiles and the Compose retrainer

- [x] 3.1 `spotter-demo-samples` scenario bundles (`test_demo_samples.py`: same tiles in every bundle, deterministic).
- [x] 3.2 `retrainer` service in the `retrain` profile (`compose_test.go`).

## 4. Demo page

- [x] 4.1 `lib/scenario.ts` reducer, `lib/explain.ts`, `lib/psi.ts`, `lib/replay.ts` (Vitest: drift ends the haze step, new version in shadow, accuracy by version, contributions add up, drift event explained, shortened wait).
- [x] 4.2 Live sender, feed and polling, recording log; `/demo` page and its panels; nav link.
- [x] 4.3 Replay mode; stub gateway and Playwright tests (static host offers the replay, approval waits for a person).

## 5. Real run

- [x] 5.1 Run the scenario on the live stack, a person approves, save the recording to `dashboard/public/recordings/demo.json`.

## 6. Docs

- [x] 6.1 README, walkthrough regenerated, specs synced and change archived.
