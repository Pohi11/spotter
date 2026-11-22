# Spec Delta

## Purpose

Defines how clients submit images for classification and read the result through the gateway. Message shapes are defined in `proto/spotter/v1/inference.proto`.

## ADDED Requirements

### Requirement: Images are submitted as asynchronous jobs
Submitting an image for a model SHALL store the image in the object store, record the job as `PENDING`, publish a `spotter.v1.InferenceRequested` event on `inference.requested.<model_name>` with message id `requested-<job_id>`, and return the job id without waiting for the result. The operation SHALL be available over gRPC and as `POST /v1/models/{model_name}/inferences`.

#### Scenario: Submit an image
- **WHEN** a client submits a 64×64 JPEG for `eurosat-classifier`
- **THEN** the response holds a new job id
- **AND** the job reads as `PENDING`, the image is in the object store, and one `InferenceRequested` event for the job is on the stream

### Requirement: Submissions are validated
A submission SHALL be rejected as invalid input, without storing or publishing anything, when the image is empty, larger than 1 MiB, or the model name is malformed. A submission for a model the registry doesn't know SHALL fail with `NOT_FOUND`, also without storing or publishing anything.

#### Scenario: Unknown model
- **WHEN** a client submits an image for `no-such-model`
- **THEN** the call fails with `NOT_FOUND`, and nothing is stored or published

#### Scenario: Empty image
- **WHEN** a client submits an empty image
- **THEN** the call fails with `INVALID_ARGUMENT` naming the `image` field, and nothing is published

#### Scenario: Oversized image
- **WHEN** a client submits a 2 MiB image
- **THEN** the call fails with `INVALID_ARGUMENT` naming the `image` field

### Requirement: Job status can be read
Reading a job SHALL return its status (`PENDING`, `SUCCEEDED`, or `FAILED`), and for a succeeded job the predicted class, its confidence, the score for every class, and the model version that produced it; for a failed job, the reason. An unknown job id SHALL fail with `NOT_FOUND`. The operation SHALL be available over gRPC and as `GET /v1/inferences/{job_id}`.

#### Scenario: Pending job
- **WHEN** a job has been submitted and not yet processed
- **THEN** reading it returns status `PENDING`

#### Scenario: Completed job
- **WHEN** a worker has completed a job
- **THEN** reading it returns `SUCCEEDED` with a label, a confidence between 0 and 1, ten class scores, and the model version id and number

#### Scenario: Unknown job
- **WHEN** a client reads a job id that was never submitted
- **THEN** the call fails with `NOT_FOUND`
