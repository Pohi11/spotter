# model-lifecycle Specification

## Purpose
Defines the stages a model version moves through, which moves are allowed, and the guarantees around each move: history, the one-production-version rule, concurrent updates, and outbox events.

## Requirements

### Requirement: Only listed stage transitions are allowed
A version's stage SHALL change only along these transitions:

| From | To |
|---|---|
| `registered` | `validated`, `rejected` |
| `validated` | `shadow`, `rejected` |
| `shadow` | `production`, `rejected` |
| `production` | `retired`, `rolled_back` |
| `retired` | `production` (restore) |

Any other transition, including one to the current stage, SHALL fail with an illegal-transition error naming both stages and change nothing.

#### Scenario: Every listed transition is allowed
- **WHEN** a version in each "From" stage is moved to each of its listed "To" stages
- **THEN** each transition succeeds

#### Scenario: Every unlisted transition is rejected
- **WHEN** a version is moved along any (from, to) pair of stages not in the table
- **THEN** the transition fails with an illegal-transition error naming both stages

#### Scenario: Skipping validation is rejected
- **WHEN** a version in `registered` is moved directly to `production`
- **THEN** the transition fails and the version stays in `registered`

### Requirement: Rejected and rolled-back versions are final
A version in `rejected` or `rolled_back` SHALL NOT move to any other stage. Retraining produces a new version instead.

#### Scenario: Terminal stages have no exits
- **WHEN** a version in `rejected` or `rolled_back` is moved to any stage
- **THEN** the transition fails with an illegal-transition error

### Requirement: A transition requires a reason
Every stage transition SHALL carry a non-blank reason.

#### Scenario: Blank reason is rejected
- **WHEN** a version is moved from `registered` to `validated` with the reason "   "
- **THEN** the transition fails with a validation error naming the `reason` field

### Requirement: Every transition is recorded in the version's history
Each successful transition SHALL append one history entry with the previous stage, the new stage, the reason, the resulting revision, and the time. A failed transition SHALL append nothing.

#### Scenario: History lists transitions in order
- **WHEN** a version is moved from `registered` to `validated` and then to `shadow`
- **THEN** its history has two entries, `registered → validated` then `validated → shadow`, with revisions 2 and 3

#### Scenario: Failed transition leaves no history
- **WHEN** a version in `registered` is moved to `retired`
- **THEN** its history is empty

### Requirement: At most one production version per model
A model SHALL have at most one version in stage `production`. Moving a second version of the same model to `production` SHALL fail with a production-occupied error and change nothing. The database SHALL enforce this independently of application code.

#### Scenario: Second production version is rejected
- **WHEN** version 1 of a model is in `production` and version 2 is moved from `shadow` to `production`
- **THEN** the transition fails with a production-occupied error
- **AND** version 2 stays in `shadow`

#### Scenario: Different models may each have a production version
- **WHEN** one version of each of two models is moved to `production`
- **THEN** both transitions succeed

#### Scenario: Database rejects a second production row directly
- **WHEN** a second `production` row for the same model is written directly to the database, bypassing application code
- **THEN** the database rejects the write

### Requirement: Stale transitions are rejected
Each version SHALL carry a revision number that increases by one on every transition. A transition SHALL state the revision it expects. If the stored revision differs, the transition SHALL fail with a conflict error and change nothing. When two transitions of one version race with the same expected revision, exactly one SHALL succeed.

#### Scenario: Stale revision is rejected
- **WHEN** a version at revision 2 is moved with expected revision 1
- **THEN** the transition fails with a conflict error

#### Scenario: Concurrent transitions, one winner
- **WHEN** two transitions of the same version at revision 1 run concurrently, one to `validated` and one to `rejected`
- **THEN** exactly one succeeds and the other fails with a conflict error
- **AND** the version's history has exactly one entry

### Requirement: Changes and their events are stored together
Registering a version SHALL store a `spotter.v1.ModelVersionRegistered` event, and each successful transition SHALL store a `spotter.v1.ModelVersionStageChanged` event, in the outbox in the same transaction as the change. Each event SHALL have a unique id. A failed registration or transition SHALL store no event. Event payloads SHALL be the protobuf messages defined in `proto/spotter/v1/registry_events.proto`.

#### Scenario: Registration stores an event
- **WHEN** a version is registered
- **THEN** the outbox holds one `spotter.v1.ModelVersionRegistered` event whose payload decodes to the version's id, model name, number, artifact URI, and checksum

#### Scenario: Transition stores an event
- **WHEN** a version is moved from `registered` to `validated`
- **THEN** the outbox holds a `spotter.v1.ModelVersionStageChanged` event whose payload decodes to the old stage, the new stage, the reason, and revision 2

#### Scenario: Failed transition stores no event
- **WHEN** a transition fails for any reason (illegal, stale, or production occupied)
- **THEN** the number of events in the outbox is unchanged
