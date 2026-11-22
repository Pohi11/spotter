# Spec Delta

## Purpose

Defines how a shadow version earns promotion to production — statistics from real traffic, gates that are code, a recorded human approval — and how production is swapped and rolled back atomically.

## ADDED Requirements

### Requirement: Shadow comparisons are counted once per job
The registry SHALL record each `ShadowCompared` event against its shadow version at most once per job id, and report for a version the number of comparisons, the number of agreeing predictions, and the mean shadow latency.

#### Scenario: Redelivered comparison
- **WHEN** the same job's comparison is delivered twice
- **THEN** the version's comparison count increases by one

#### Scenario: Statistics
- **WHEN** a shadow version has 3 comparisons, 2 agreeing, with shadow latencies 2, 4, and 6 ms
- **THEN** its statistics report 3 comparisons, 2 agreeing, and a mean latency of 4 ms

### Requirement: Promotion gates are evaluated as code
For a candidate in `shadow`, the gates SHALL be: `accuracy` (holdout accuracy at least production's minus a tolerance, default 0.01; passes if there is no production version), `shadow_agreement` (at least 200 comparisons and an agreement rate of at least 0.95), `shadow_latency` (mean shadow latency at most a budget, default 25 ms), and `approval` (at least one approval recorded for the version). Each gate SHALL report whether it passed and why.

#### Scenario: All gates pass
- **WHEN** the candidate's accuracy is 0.9657 against production's 0.9654, it has 500 comparisons with 497 agreeing at a mean of 3 ms, and it has an approval
- **THEN** every gate passes

#### Scenario: Too few comparisons
- **WHEN** the candidate has 120 comparisons, all agreeing
- **THEN** the `shadow_agreement` gate fails with a reason stating 120 of at least 200

#### Scenario: No approval
- **WHEN** every other gate passes and no approval is recorded
- **THEN** the `approval` gate fails

### Requirement: Approvals are recorded
Approving a version SHALL record the approver's name, the reason, and the time. The approver and reason SHALL be non-blank, and only a version in `shadow` SHALL be approvable.

#### Scenario: Approve a shadow version
- **WHEN** `ana` approves a shadow version with reason "agreement 99.4% over 500 jobs"
- **THEN** the approval is stored and the `approval` gate passes

#### Scenario: Approving a validated version
- **WHEN** someone approves a version in `validated`
- **THEN** the call fails with a failed-precondition error

### Requirement: Promotion is atomic and gated
Promoting a shadow version SHALL fail with a failed-precondition error listing the failing gates unless every gate passes. When they pass, the current production version (if any) SHALL move to `retired` and the candidate to `production` in one transaction, with both history entries and both outbox events.

#### Scenario: Gated promotion refused
- **WHEN** a candidate is promoted with the `approval` gate failing
- **THEN** the call fails, names the `approval` gate, and no version changes stage

#### Scenario: Successful promotion
- **WHEN** a candidate whose gates all pass is promoted while another version is in production
- **THEN** the old version is `retired`, the candidate is `production`, and at no point is there zero or two production versions

### Requirement: Rollback restores the previous production version atomically
Rolling back a model SHALL move its production version to `rolled_back` and restore the version that was most recently retired from production, in one transaction. A model with no production version, or none previously retired from production, SHALL fail with a failed-precondition error and change nothing.

#### Scenario: Rollback after a promotion
- **WHEN** version 2 was promoted over version 3 and the model is rolled back
- **THEN** version 2 is `rolled_back` and version 3 is `production`

#### Scenario: Nothing to roll back to
- **WHEN** a model's only production version has never replaced another
- **THEN** rollback fails and nothing changes

### Requirement: Promotion operations are available over gRPC and REST
The registry SHALL serve `ApproveVersion`, `GetPromotionGates`, `PromoteVersion`, and `RollbackModel`, and the gateway SHALL expose them at `POST /v1/versions/{version_id}:approve`, `GET /v1/versions/{version_id}/gates`, `POST /v1/versions/{version_id}:promote`, and `POST /v1/models/{model_name}:rollback`.

#### Scenario: Promote over REST
- **WHEN** a client approves and promotes a shadow version through the gateway
- **THEN** both calls return 200 and the version is `production`
