# failure-lab Specification

## Purpose
Defines how the platform's behavior under faults is measured and compared with a synchronous chain.

## Requirements

### Requirement: A run injects one fault on a schedule and heals it
A chaos run SHALL drive closed-loop load for a configured duration, inject its fault at a configured offset, and heal it at a later offset. The faults SHALL be: `worker-kill` (kill the worker process without warning and start a new one at heal), `nats-restart` (restart the NATS container at the fault offset), `postgres-pause` and `objectstore-pause` (pause the container at the fault offset and unpause it at heal), and `none`.

#### Scenario: Worker kill
- **WHEN** a run with `worker-kill` reaches the fault offset
- **THEN** the worker process is killed, and at the heal offset a new worker is started

#### Scenario: Pause and unpause
- **WHEN** a run with `postgres-pause` reaches the fault and heal offsets
- **THEN** the pause command runs at the first and the unpause command at the second

### Requirement: Each run reports what clients and the system experienced
A run SHALL report requests sent, requests rejected (an error answer, whether from submission or a synchronous call), completed jobs, lost jobs (accepted but never completed by the end of the wait), duplicate completions (jobs with more than one stored completion event), p50, p99, and maximum latency of completed jobs, recovery time (from heal until the first second whose completions reach 80% of the mean before the fault), and completions and rejections per second.

#### Scenario: Lost and duplicate jobs
- **WHEN** of four accepted jobs one has two stored completions and one has none by the end
- **THEN** the report counts three completed, one lost, and one duplicate

#### Scenario: Recovery
- **WHEN** completions run at 50 per second before the fault, drop to zero, and reach 45 per second in the third second after heal
- **THEN** the recovery time is three seconds

#### Scenario: Never recovers
- **WHEN** completions never reach 80% of the pre-fault mean after heal
- **THEN** the report says the run did not recover

### Requirement: Results are recorded with how they were measured
Each mode and fault SHALL be written to a dated JSON report with the run's configuration, the machine, and the timeline, and the conclusion SHALL be recorded in a decision record that cites those numbers.

#### Scenario: Report contents
- **WHEN** a run finishes
- **THEN** its JSON holds the mode, fault, offsets, concurrency, duration, the report fields, and the per-second timeline
