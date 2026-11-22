# model-retraining Specification

## Purpose
Defines how a new model version is produced from an existing one when live inputs drift, and what retraining may do to the registry.

## Requirements

### Requirement: Retraining fine-tunes with synthetic degradations
Retraining SHALL fine-tune the newest PyTorch version on the training split only. Besides the random flips, each training tile SHALL get, with a configured probability (default 0.5), one of the drift experiment's degradations (blur, noise, haze, darken) at a strength drawn from [0.25, 1]. Validation and holdout tiles SHALL never be degraded during training.

#### Scenario: No degradation at probability zero
- **WHEN** the degradation probability is 0
- **THEN** every augmented tile equals the original tile up to flips

#### Scenario: Every tile degraded at probability one
- **WHEN** the degradation probability is 1
- **THEN** every augmented tile differs from the original and from any flip of it

#### Scenario: Same seed, same tiles
- **WHEN** augmentation runs twice with the same seed
- **THEN** it produces identical tiles

### Requirement: A retrained version is evaluated on clean and degraded holdout tiles
After training, retraining SHALL evaluate the new version, and the version it started from, on the clean holdout split and on the holdout split with each degradation at full strength, and SHALL record the new version's results as metrics `accuracy` (clean) and `accuracy_<degradation>`.

#### Scenario: Metrics recorded
- **WHEN** a retrained version is registered
- **THEN** its metrics include `accuracy`, `accuracy_blur`, `accuracy_noise`, `accuracy_haze`, and `accuracy_darken`, and its metadata holds the same figures for the version it started from

### Requirement: A retrained version reaches shadow at most
A retrained version SHALL be published with metadata naming its parent version and, when a drift event triggered it, that event's version and drifted features. It SHALL be registered, moved to `validated` only if its clean holdout accuracy meets the floor, and then moved to `shadow`. Retraining SHALL never request any other stage.

#### Scenario: Accurate enough
- **WHEN** the retrained version's clean accuracy is 0.95 and the floor is 0.85
- **THEN** the version ends in `shadow`, and the only transitions requested were to `validated` and `shadow`

#### Scenario: Below the floor
- **WHEN** the retrained version's clean accuracy is 0.80 and the floor is 0.85
- **THEN** the version stays `registered` and no transition is requested

### Requirement: Each drift event triggers one retraining
In watch mode, retraining SHALL consume `monitor.drift.detected.<model>` with a durable consumer that allows one unacknowledged event at a time, run one retraining per event, and acknowledge the event only after the run finishes. A run that fails SHALL leave the event unacknowledged for redelivery.

#### Scenario: Two drift events
- **WHEN** two `DriftDetected` events are published
- **THEN** two retraining runs happen, one after the other, each given its own event

#### Scenario: Failed run
- **WHEN** a retraining run raises an error
- **THEN** the event is not acknowledged and is delivered again

### Requirement: The chart runs retraining on demand or on drift
The chart's trainer Job SHALL run the command in `trainer.command` (default `spotter-train`), and `retrainer.enabled=true` SHALL add a one-replica Deployment running `spotter-retrain --watch` with the NATS, registry, and object-store settings. Neither SHALL render by default.

#### Scenario: Retraining Job
- **WHEN** the chart is rendered with `trainer.enabled=true` and `trainer.command=spotter-retrain`
- **THEN** the Job's container runs `spotter-retrain`

#### Scenario: Drift watcher
- **WHEN** the chart is rendered with `retrainer.enabled=true`
- **THEN** it contains a Deployment with one replica running `spotter-retrain --watch` with `NATS_URL` and `REGISTRY_ADDR` set
