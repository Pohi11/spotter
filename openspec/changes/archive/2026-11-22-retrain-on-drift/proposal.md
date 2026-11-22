# Proposal

## Why

The monitor detects drift and the registry can shadow, gate, and promote a new version, but nothing produces that new version. The loop the platform exists for (drift detected, a model trained for the shifted inputs, shadowed, approved, promoted) stops at the alarm.

## What Changes

- `spotter-retrain` fine-tunes the newest PyTorch version of a model on the training split with synthetic degradations (the drift experiment's blur, noise, haze, and darken at random strengths) added to the usual flips, evaluates it on the clean holdout and on each degradation of the holdout, publishes it with its lineage, registers it, validates it against the accuracy floor, and moves it to shadow. It never moves anything to production.
- `spotter-retrain --watch` waits on `monitor.drift.detected.<model>` with a durable consumer and runs one retraining per drift event, recording the event in the new version's metadata.
- The Helm chart's trainer Job can run either `spotter-train` or `spotter-retrain`; an optional retrainer Deployment runs the watch mode.

## Capabilities

### New Capabilities
- `model-retraining`: how a new version is produced from an existing one when inputs drift, and what it may and may not do to the registry.

### Modified Capabilities
None.

## Impact

- **Trainer package:** new modules and console script; no change to `spotter-train`.
- **Chart:** `trainer.command` and an optional `retrainer` Deployment.
- **Decision record:** 0018 (retrain by fine-tuning with synthetic degradations, triggered by drift, shadow only).
- **Out of scope:** retraining on real labels from production (there are none; all data is public or synthetic), and automatic promotion (promotion still needs the gates and a person).
