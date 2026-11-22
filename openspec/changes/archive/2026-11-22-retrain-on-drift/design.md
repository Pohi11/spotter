# Design

## Context

The drift monitor raises `DriftDetected` from unlabeled traffic (change `phase-07-drift-monitor`), and promotion needs a candidate in shadow with gates and an approval (change `phase-08-promotion`). There are no labels for live traffic, and every input is public or synthetic, so a retrained model can only learn from the labeled training split. The approach is recorded in decision record [0018](../../../../docs/decisions/0018-retrain-by-fine-tuning-with-synthetic-degradations.md).

## Decisions

### D1. Fine-tune, don't train from scratch
The newest PyTorch version's weights are the starting point (ONNX variants can't be trained). A few epochs (default 3) at a lower learning rate (default 5e-4) adapt it in minutes on a laptop CPU, against about 18 minutes from scratch. The split, seed, and normalization are the parent's, read from its `metadata.json`, so the holdout stays the same tiles and the parent's numbers are comparable.

### D2. Degradations as augmentation
`spotter_trainer.retrain.augment` applies the existing flips and then, per tile with probability `DEGRADE_P`, one of `degrade.KINDS` at a strength drawn from [0.25, 1], through `degrade.degrade` on the decoded tile, so the training distribution covers what the monitor flags. A seeded generator makes runs repeatable.

### D3. Robustness is measured, not assumed
The holdout split is decoded after training and evaluated clean and with each degradation at strength 1.0, for both the new and the parent version. The registry gets the new version's numbers as metrics; the metadata holds both.

### D4. Shadow is as far as retraining goes
Retraining registers, validates against the floor, and transitions to `shadow`. Promotion stays where it was: gates in code plus a recorded approval. A model's output never moves anything into production.

### D5. Watch mode
`spotter-retrain --watch` creates or binds the durable consumer `retrainer-<model>` on the `MONITOR` stream, filtered to `monitor.drift.detected.<model>`, with `max_ack_pending=1` and an ack wait longer than a run (default 1 hour), so events queue behind a running retrain. A run is acked when it finishes and naked on error. The run function is injected, so the consumer logic is tested against real NATS without training.

### D6. Packaging
The trainer Job's command comes from `trainer.command` (default `spotter-train`). `retrainer.enabled` adds a one-replica Deployment running `spotter-retrain --watch`.

## Risks and trade-offs

- **Synthetic degradations are a guess at the shift.** A version trained on them may not help with a shift of another kind; the shadow agreement and the per-degradation metrics show it before anyone approves it.
- **Clean accuracy can drop.** The accuracy gate compares with production and the approval is a person's; both see the clean number.
- **Retraining holds the dataset and a model in memory** (about 1 GB of decoded tiles), so the retrainer Deployment needs the trainer's resources while it runs.
