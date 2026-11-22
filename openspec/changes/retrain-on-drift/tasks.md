# Tasks

## 1. Decision

- [x] 1.1 Decision record 0018.

## 2. Retraining

- [ ] 2.1 `retrain.augment` with degradations (`test_retrain.py`: probability 0, probability 1, same seed).
- [ ] 2.2 Clean and degraded holdout evaluation; publish, register, validate, shadow (`test_retrain.py` with a fake registry: accurate enough, below the floor, metrics and metadata).
- [ ] 2.3 `spotter-retrain` CLI and watch mode (`test_retrain_watch.py` against real NATS: two events, failed run).

## 3. Packaging and real run

- [ ] 3.1 Chart: `trainer.command`, optional retrainer Deployment (chart test).
- [ ] 3.2 Real run: drift on the live stack, retrain, the new version in shadow with its measured metrics.
