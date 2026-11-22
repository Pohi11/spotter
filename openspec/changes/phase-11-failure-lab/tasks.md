# Tasks

## 1. Synchronous baseline

- [ ] 1.1 `classifier.proto` and `ClassifySync` in `inference.proto`; worker gRPC server and `SHADOW` (pytest against an in-process server).
- [ ] 1.2 Gateway `ClassifySync` use case, `Classifier` port, gRPC adapter, `WORKER_ADDR` (Go tests: classified, not configured, worker down, unknown model).

## 2. Chaos runner

- [ ] 2.1 `internal/chaos`: schedule, report arithmetic (`chaos_test.go`: fault and heal calls, lost and duplicate counts, recovery, never recovers).
- [ ] 2.2 `cmd/spotterchaos`: worker process control, Docker faults, async and sync drivers, JSON report.

## 3. Measurements

- [ ] 3.1 Run both modes under every fault; dated report; decision record 0019.
