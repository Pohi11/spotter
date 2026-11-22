# Design

## Context

The inference path's guarantees are tested in pieces: the worker stopped before recording and before acknowledging still yields one result per job (change `phase-04-event-driven-inference`), a rollback under load lost nothing (change `phase-08-promotion`). The failure lab measures the whole running system under faults and compares it with the design it replaced on paper: the gateway calling the worker directly. The conclusion is recorded in decision record [0019](../../../../docs/decisions/0019-event-driven-inference-under-faults.md).

## Decisions

### D1. The synchronous baseline is a real path, not a simulation
The worker serves `ClassifierService/Classify` with the same model and code as the bus path (`processor.classify`). The gateway's `ClassifySync` use case checks the model with the registry, as submission does, then calls a `Classifier` port whose adapter is a gRPC client to `WORKER_ADDR`. So both modes share the gateway, the registry check, the worker, and the model; only the hand-off differs.

### D2. Same work per job
With a shadow version loaded, the bus path classifies twice per job. `SHADOW=off` makes the worker ignore shadow versions, so the comparison isn't skewed by work only one mode does.

### D3. The chaos runner owns the worker
`spotterchaos` starts the worker as a child process (`-worker-cmd`), so `worker-kill` can kill it without warning and start a fresh one at heal, as a crashed pod would be. Container faults run Docker commands against the Compose containers. The runner's scheduling and report arithmetic live in `internal/chaos` and are unit-tested with fakes; the commands are configuration.

### D4. What "lost" and "duplicate" mean
A request rejected with an error is visible to its client, which can retry; it is reported as rejected. A job accepted with a job id that never completes is lost: no one will tell the client. A duplicate is a job with more than one completion stored on the `INFERENCE` stream, which a downstream consumer would see twice. Completions are read with an ordered consumer from the run's start time, not a plain subscription, because only stored messages reach consumers.

### D5. Recovery
Recovery is measured from the heal offset to the first one-second bucket whose completions reach 80% of the mean rate before the fault. A run that never gets there is reported as not recovered.

## Risks and trade-offs

- **One laptop, everything local.** The numbers say how each design degrades relative to itself on the same machine, not absolute capacity.
- **The synchronous baseline has no retries.** A client of a real synchronous service would retry; the report counts its rejections, which are what a client would have to retry.
