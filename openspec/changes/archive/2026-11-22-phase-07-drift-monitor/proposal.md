# Proposal

## Why

A deployed model degrades silently when the images it sees stop resembling the ones it was trained on — a sensor change, haze, night passes, compression. Labels arrive late or never, so accuracy can't be watched directly. The platform needs to notice, from unlabeled traffic, that inputs or the model's confidence have shifted away from the training distribution, and announce it as an event other services (and later, retraining) can act on.

## What Changes

- Every published model version carries a `reference.json`: per-image features (per-channel brightness, contrast, model confidence) computed over the validation split. The trainer and the optimizer write it; a `spotter-reference` command adds it to existing versions.
- Add the drift monitor, a Go service that consumes `InferenceCompleted` events, keeps a sliding window of recent features per model version, and compares the window against that version's reference with the population stability index (PSI) and the two-sample Kolmogorov–Smirnov test.
- Publish `spotter.v1.DriftDetected` when any feature's PSI reaches 0.2, and `spotter.v1.DriftResolved` when every feature falls back below 0.1, once per transition, on a new `MONITOR` stream.
- Expose each feature's latest PSI as a Prometheus gauge.
- Add `spotter-degrade` to produce degraded copies of holdout tiles (darkening, blur, noise, haze) at chosen strengths, and measure detection time and false alarms.

## Capabilities

### New Capabilities
- `drift-monitoring`: The statistics, the detection rule, the events, the reference, and the monitor process.

### Modified Capabilities
- `model-publishing`: each published version carries a reference distribution.
- `event-streams`: a `MONITOR` stream for `monitor.>` subjects.

## Impact

- **New service:** `services/monitor` (hexagonal Go).
- **Proto:** `monitor.proto` (drift events).
- **Python:** `reference.py` and `spotter-reference`, `spotter-degrade` in the trainer package.
- **Out of scope:** concept drift (needs labels), automatic retraining (a later change), per-class confidence drift, alert routing.
