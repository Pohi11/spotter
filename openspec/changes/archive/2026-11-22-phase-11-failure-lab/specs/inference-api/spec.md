# Spec Delta

## Purpose

Adds a synchronous classification path, used as the baseline in the failure lab.

## ADDED Requirements

### Requirement: Synchronous classification calls the worker directly
When `WORKER_ADDR` is set, `ClassifySync` (`POST /v1/models/{model_name}:classify`) SHALL check that the model exists and pass the image to the worker's `ClassifierService/Classify` with the caller's deadline, returning the prediction in the same call without storing the image, recording a job, or publishing an event. Without `WORKER_ADDR` it SHALL answer `UNIMPLEMENTED`. A worker that can't be reached or has no production version loaded SHALL give `UNAVAILABLE`.

#### Scenario: Classified in one call
- **WHEN** `WORKER_ADDR` points at a worker serving version 3 and a valid image is sent
- **THEN** the response holds the prediction from version 3, and no job and no event exist for it

#### Scenario: Not configured
- **WHEN** the gateway runs without `WORKER_ADDR`
- **THEN** `ClassifySync` answers `UNIMPLEMENTED`

#### Scenario: Worker down
- **WHEN** the worker can't be reached
- **THEN** `ClassifySync` answers `UNAVAILABLE`

#### Scenario: Unknown model
- **WHEN** the model doesn't exist
- **THEN** `ClassifySync` answers `NOT_FOUND` and the worker is not called
