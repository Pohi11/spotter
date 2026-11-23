# Proposal

## Why

Spotter builds its serving, registry, monitoring, promotion, and messaging from small parts. Each has a mature off-the-shelf alternative, and the measurements gathered while building it (load tests, the failure lab, promotion and rollback timing) make it possible to say concretely what adopting those alternatives would have given and cost, instead of arguing from preference.

## What Changes

- Decision record 0021 compares the design with KServe, Triton, and BentoML (serving), MLflow (tracking and registry), Kafka and RabbitMQ (messaging), and Temporal (workflows), citing the measured numbers, and states when each would be worth adopting.
- No code or behavior changes.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
None.

## Impact

- **New:** `docs/decisions/0021-retrospective-off-the-shelf-alternatives.md`.
