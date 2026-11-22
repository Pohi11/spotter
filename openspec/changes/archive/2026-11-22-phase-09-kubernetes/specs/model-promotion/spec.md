# Spec Delta

## Purpose

Lets the first version of a model reach production: with no production version, the worker has nothing to shadow against.

## MODIFIED Requirements

### Requirement: Promotion gates are evaluated as code
For a candidate in `shadow`, the gates SHALL be: `accuracy` (holdout accuracy at least production's minus a tolerance, default 0.01), `shadow_agreement` (at least 200 comparisons and an agreement rate of at least 0.95), `shadow_latency` (mean shadow latency at most a budget, default 25 ms), and `approval` (at least one approval recorded for the version). Each gate SHALL report whether it passed and why.

#### Scenario: All gates pass
- **WHEN** the candidate's accuracy is 0.9657 against production's 0.9654, it has 500 comparisons with 497 agreeing at a mean of 3 ms, and it has an approval
- **THEN** every gate passes

#### Scenario: Too few comparisons
- **WHEN** a production version exists and the candidate has 120 comparisons, all agreeing
- **THEN** the `shadow_agreement` gate fails with a reason stating 120 of at least 200

#### Scenario: No approval
- **WHEN** every other gate passes and no approval is recorded
- **THEN** the `approval` gate fails

## ADDED Requirements

### Requirement: A model's first version is promoted on approval
When the model has no production version, the `accuracy`, `shadow_agreement`, and `shadow_latency` gates SHALL pass with a reason saying there is no production version to compare with, so that the approval decides.

#### Scenario: First version of a model
- **WHEN** the model has no production version and the candidate has no comparisons
- **THEN** only the `approval` gate can fail, and the shadow gates say there is no production version to compare with
