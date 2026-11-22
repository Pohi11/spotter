# Spec Delta

## Purpose

Defines how Spotter detects, from unlabeled inference traffic, that a model version's inputs or confidence have shifted away from its training-time reference, and how it announces the shift.

## ADDED Requirements

### Requirement: PSI compares a window to the reference
The population stability index of a feature SHALL be computed over 10 bins whose edges are the reference's deciles (the outer bins open-ended), as the sum over bins of (current share − reference share) × ln(current share ÷ reference share), with every share floored at 0.0001 so empty bins stay finite.

#### Scenario: Same distribution
- **WHEN** the window is drawn from the same distribution as a reference of 4,000 values
- **THEN** PSI is below 0.05

#### Scenario: Shifted distribution
- **WHEN** every window value is the reference value plus three reference standard deviations
- **THEN** PSI is above 1.0

#### Scenario: Empty bins stay finite
- **WHEN** all window values fall into a single bin
- **THEN** PSI is a finite number

### Requirement: The KS test is reported with each comparison
Each comparison SHALL also report the two-sample Kolmogorov–Smirnov statistic D (the largest gap between the two empirical distribution functions) and its asymptotic p-value.

#### Scenario: Known statistic
- **WHEN** the reference is 1, 2, 3, 4 and the window is 3, 4, 5, 6
- **THEN** D is 0.5

#### Scenario: Identical samples
- **WHEN** the window equals the reference
- **THEN** D is 0 and the p-value is 1

### Requirement: Drift events describe the shifted features
A `spotter.v1.DriftDetected` event SHALL name each feature whose PSI is at least 0.2, with its PSI, KS statistic, KS p-value, reference mean, and window mean.

#### Scenario: Event contents
- **WHEN** drift is detected because brightness shifted
- **THEN** the event lists the brightness features with PSI at least 0.2 and their reference and window means

### Requirement: Drift is announced once per change of state
Once a version's window holds 200 events, the monitor SHALL evaluate it every 50 events. It SHALL publish one `DriftDetected` on `monitor.drift.detected.<model>` when any feature's PSI reaches 0.2 while not drifting, and one `DriftResolved` on `monitor.drift.resolved.<model>` when every PSI falls below 0.1 while drifting. Other evaluations SHALL publish nothing.

#### Scenario: Clean traffic
- **WHEN** a version receives events drawn from its reference distribution
- **THEN** no drift event is published

#### Scenario: Darkened images
- **WHEN** a version receives events from images darkened to half brightness
- **THEN** one `DriftDetected` is published naming the brightness features

#### Scenario: Drift persists
- **WHEN** drifted events keep arriving for several more evaluations
- **THEN** no further `DriftDetected` is published

#### Scenario: Traffic returns to normal
- **WHEN** drifted traffic is followed by enough clean traffic to fill the window
- **THEN** one `DriftResolved` is published

### Requirement: Each version is compared with its own reference
The monitor SHALL keep a separate window and state for each model version, keyed by the version id in each event, and compare it with that version's `reference.json`. Events for a version without a reference SHALL be counted and skipped, not compared with another version's reference.

#### Scenario: Two versions at once
- **WHEN** events from two versions of a model interleave
- **THEN** each version's window holds only its own events

#### Scenario: Missing reference
- **WHEN** events arrive for a version that has no `reference.json`
- **THEN** they are skipped and counted, and no drift event is published for that version

### Requirement: The monitor process is configured from the environment and observable
The monitor SHALL read `NATS_URL`, `REGISTRY_ADDR`, `S3_ENDPOINT`, `S3_ACCESS_KEY`, and `S3_SECRET_KEY` (required) and `HTTP_ADDR` (default `:8082`) and `SHUTDOWN_TIMEOUT` (default `10s`), failing startup with an error naming any missing variable. It SHALL serve `/livez`, `/readyz` (NATS connected), and `/metrics` with a gauge of the latest PSI per model, version, and feature, and shut down gracefully on SIGINT or SIGTERM.

#### Scenario: Missing NATS URL
- **WHEN** the monitor starts without `NATS_URL`
- **THEN** startup fails with an error naming `NATS_URL`

#### Scenario: PSI gauge
- **WHEN** a version has been evaluated
- **THEN** `/metrics` includes `spotter_monitor_psi` for each of its features
