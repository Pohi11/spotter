# Spec Delta

## Purpose

Defines how the system is driven with concurrent inference requests and what the load generator reports.

## ADDED Requirements

### Requirement: The load generator measures end-to-end latency from completion events
The load generator SHALL submit images through the gateway from a configured number of concurrent clients for a configured duration, time each job from just before submission to the arrival of its `InferenceCompleted` event, and report completed jobs, failed submissions, jobs that never completed within a timeout, throughput, and p50, p95, p99, and maximum latency.

#### Scenario: Every job completes
- **WHEN** the load generator runs against a system that completes every job
- **THEN** the report counts every submitted job as completed, with latency percentiles and throughput

#### Scenario: Some jobs never complete
- **WHEN** some submitted jobs never produce a completion event within the timeout
- **THEN** the report counts them as lost and excludes them from the latency percentiles

#### Scenario: Submission errors
- **WHEN** the gateway rejects some submissions
- **THEN** the report counts them as failed submissions
