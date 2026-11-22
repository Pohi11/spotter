# shadow-inference Specification

## Purpose
Defines how the worker runs a candidate version on real traffic without affecting results, and what it reports.

## Requirements

### Requirement: The worker runs the shadow version without affecting results
When a version of its model is in `shadow`, the worker SHALL load the newest such version (verifying its checksum like production) and, for every job, classify the image with both versions. The job's recorded result and completion event SHALL come from production only. A failure of the shadow model SHALL NOT change the job's outcome.

#### Scenario: Shadow loaded
- **WHEN** a model has a production and a shadow version
- **THEN** each job's result reports the production version, and a comparison is published for the shadow version

#### Scenario: Shadow fails
- **WHEN** the shadow model raises an error on a job
- **THEN** the job still succeeds with production's result

### Requirement: Comparisons are published once per job
For each job classified by both versions, the worker SHALL publish `spotter.v1.ShadowCompared` on `inference.shadow.<model_name>` with message id `shadow-<job_id>`, carrying both version ids, both labels, whether they agree, and both model latencies.

#### Scenario: Comparison contents
- **WHEN** a job is classified by production and shadow
- **THEN** one `ShadowCompared` event for the job carries both labels, both latencies, and `agree` equal to whether the labels match
