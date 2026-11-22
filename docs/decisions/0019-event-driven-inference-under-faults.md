# 0019 — Keep event-driven inference as the default path; measured under faults

- **Date:** 2026-11-22
- **Status:** Accepted. Confirms [0009](0009-at-least-once-inference-with-kv-job-state.md) with measurements.

## Context

Inference goes through the bus: the gateway stores the tile, records a pending job, and publishes it; a worker pulls it, records one result, publishes a completion, and acknowledges last ([0009](0009-at-least-once-inference-with-kv-job-state.md)). The alternative is a synchronous chain: the gateway calls the worker over gRPC and answers in the same call. The failure lab ran both against the same gateway, registry check, worker, and model, under five conditions ([benchmarks/2026-11-22-failure-lab.md](../../benchmarks/2026-11-22-failure-lab.md)).

## Options considered

1. **Bus only** (as built).
2. **Direct call only:** simpler and faster, one fewer service on the path.
3. **Bus by default, direct call available** for callers that need the lowest latency and can retry.

What was measured, 4 clients, 40 s per run:

| | Bus | Direct call |
|---|---|---|
| Throughput without faults | 61.9 jobs/s | 435.2 requests/s |
| Median latency without faults | 64 ms | 9 ms |
| Accepted jobs lost, any fault | 0 | 0 (nothing is accepted without an answer) |
| Duplicate completions, any fault | 0 | not applicable |
| Requests failed when the worker was killed for 10 s | **0** (one job delayed 30 s by the acknowledgement deadline) | **1,204**: every request until a new worker was ready |
| NATS restart | stalled about 4 s, half rate about 5 s more, 2 failed | unaffected |
| Object store paused 10 s | stalled 10 s, 3 failed at the client deadline | unaffected |
| Postgres paused 10 s | stalled 10 s, none failed | stalled 10 s, none failed |

## Decision

Option 3. The bus stays the default path: a worker crash, the fault most likely in a fleet of model servers, became a delay instead of failed requests, and no accepted job was lost or completed twice under any fault. The direct call stays available behind `WORKER_ADDR` for callers that value latency over buffering and can retry.

Two follow-ups come straight from the numbers:

- **Shorter redelivery after a crash.** A job held by a dead worker waits the full `ACK_WAIT` (30 s). Lowering it, with the worker sending in-progress acknowledgements for slow jobs, would cut that delay.
- **Fewer dependencies on the submit path.** The object store costs most of the bus path's latency and stalls submission when it stalls. Small tiles (about 3 KB) could travel inside the event instead, keeping the store for large images only.

## Consequences

- **Good:** The guarantees claimed in 0009 hold for the whole running system under faults, with numbers.
- **Cost:** About a seventh of the direct call's throughput and 55 ms more median latency on this machine, and two more dependencies (NATS and the object store) that each stall submission when they stall.
- **Cost:** Two inference paths to keep consistent; both share the same use-case code for the model check and the same worker code for classification, which limits the drift.
