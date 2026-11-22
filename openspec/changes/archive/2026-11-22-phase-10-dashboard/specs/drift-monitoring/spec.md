# Spec Delta

## Purpose

Publishes every drift evaluation, so drift statistics can be followed over time from the bus.

## MODIFIED Requirements

### Requirement: Drift is announced once per change of state
Once a version's window holds 200 events, the monitor SHALL evaluate it every 50 events. It SHALL publish one `DriftDetected` on `monitor.drift.detected.<model>` when any feature's PSI reaches 0.2 while not drifting, and one `DriftResolved` on `monitor.drift.resolved.<model>` when every PSI falls below 0.1 while drifting. Other evaluations SHALL publish neither event.

#### Scenario: Clean traffic
- **WHEN** a version receives events drawn from its reference distribution
- **THEN** no `DriftDetected` or `DriftResolved` is published

#### Scenario: Darkened images
- **WHEN** a version receives events from images darkened to half brightness
- **THEN** one `DriftDetected` is published naming the brightness features

#### Scenario: Drift persists
- **WHEN** drifted events keep arriving for several more evaluations
- **THEN** no further `DriftDetected` is published

#### Scenario: Traffic returns to normal
- **WHEN** drifted traffic is followed by enough clean traffic to fill the window
- **THEN** one `DriftResolved` is published

## ADDED Requirements

### Requirement: Every evaluation is published
At every evaluation the monitor SHALL publish one `DriftEvaluated` on `monitor.drift.evaluated.<model>` holding every feature's comparison, the window size, whether the version is drifting after this evaluation, and the evaluation time, with message id `evaluated-<version id>-<stream sequence of the triggering completion>` so a redelivered completion doesn't publish it twice.

#### Scenario: Evaluations on clean traffic
- **WHEN** a version receives 300 events drawn from its reference distribution
- **THEN** three `DriftEvaluated` events are published (at 200, 250, and 300 events), each listing all five features and saying the version is not drifting

#### Scenario: Evaluation that detects drift
- **WHEN** an evaluation detects drift
- **THEN** its `DriftEvaluated` says the version is drifting, alongside the one `DriftDetected`
