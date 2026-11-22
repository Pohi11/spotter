# Spec Delta

## Purpose

Defines how the inference worker turns `InferenceRequested` events into results under at-least-once delivery: which model it uses, how it retries, how it guarantees one result per job, and what happens to jobs that can never succeed.

## ADDED Requirements

### Requirement: The worker serves its model's production version
The worker SHALL load the version of its configured model that is in stage `production`, downloading its artifact from the object store and refusing to load it unless the artifact's SHA-256 equals the checksum registered for the version. While no production version is loaded, jobs SHALL be retried later rather than failed immediately.

#### Scenario: Production version loaded
- **WHEN** the worker starts and the model has a production version
- **THEN** it loads that version and reports its id and number with every result

#### Scenario: Checksum mismatch
- **WHEN** the stored artifact's SHA-256 differs from the registered checksum
- **THEN** the worker does not load it and processes no jobs with it

#### Scenario: No production version yet
- **WHEN** a job arrives and no production version is loaded
- **THEN** the job is negatively acknowledged with a delay and delivered again later

### Requirement: Each job produces exactly one result
Delivery is at least once, so the worker SHALL record a job's result only if the job is still `PENDING`, publish `spotter.v1.InferenceCompleted` on `inference.completed.<model_name>` with message id `completed-<job_id>`, and acknowledge the message only after both. A redelivered job whose result is already recorded SHALL be acknowledged without recording another result, after re-publishing its completion event.

#### Scenario: Successful job
- **WHEN** a worker processes a job
- **THEN** the job reads as `SUCCEEDED` and exactly one `InferenceCompleted` event for it is on the stream

#### Scenario: Worker stops before acknowledging
- **WHEN** a worker records a result and stops before acknowledging, and the job is delivered again after the acknowledgement wait
- **THEN** the job still has exactly one recorded result and the stream holds exactly one `InferenceCompleted` event for it

#### Scenario: Worker stops before recording
- **WHEN** a worker receives a job and stops before recording a result
- **THEN** the job is delivered again and completed

### Requirement: Jobs that cannot succeed are dead-lettered
A job whose event cannot be decoded, whose image cannot be decoded, or that fails on its last allowed delivery SHALL be terminated (not redelivered), recorded as `FAILED` with a reason when its job id is known, and published as `spotter.v1.InferenceFailed` on `inference.failed.<model_name>`. Transient failures before the last delivery SHALL be retried with a delay.

#### Scenario: Corrupt image
- **WHEN** a job's stored image is not a decodable image
- **THEN** the job reads as `FAILED` with a reason, an `InferenceFailed` event is published, and the message is not delivered again

#### Scenario: Undecodable event
- **WHEN** a message on `inference.requested.<model_name>` is not a valid `InferenceRequested`
- **THEN** it is terminated and an `InferenceFailed` event is published

#### Scenario: Retries exhausted
- **WHEN** a job keeps failing transiently until its last allowed delivery
- **THEN** it is recorded as `FAILED` and an `InferenceFailed` event is published

### Requirement: The worker follows production changes
When a `ModelVersionStageChanged` event shows a version of the worker's model entering or leaving `production`, the worker SHALL reload so that subsequent results come from the current production version.

#### Scenario: Promotion while running
- **WHEN** a new version of the model is moved to `production` while the worker runs
- **THEN** jobs processed afterwards report the new version
