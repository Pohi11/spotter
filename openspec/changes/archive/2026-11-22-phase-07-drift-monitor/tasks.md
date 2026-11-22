# Tasks

## 1. Reference distributions

- [x] 1.1 Write decision record 0012. Add `reference.py`, write `reference.json` from the trainer and optimizer, and add `spotter-reference`. Verify the `model-publishing` reference scenarios.

## 2. Monitor domain

- [x] 2.1 Add features, PSI, KS, and the detector to `services/monitor/internal/domain`. Verify the PSI, KS, and state scenarios.

## 3. Monitor service

- [x] 3.1 Add `monitor.proto`, the `MONITOR` stream, ports, app, adapters, and `cmd/monitor`. Verify the per-version, missing-reference, process, and event scenarios.

## 4. Experiments

- [x] 4.1 Add `spotter-degrade`. Run clean and degraded traffic through the local stack; record detection times and false alarms.
