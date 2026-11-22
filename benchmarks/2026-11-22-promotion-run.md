# Shadow, gated promotion, and rollback: real run (2026-11-22)

The full promotion path against the local stack: registry, gateway, and inference worker on the host (Windows 11, 22 logical processors), with NATS, SeaweedFS, and Postgres in Docker Desktop. Traffic came from `spotterload`: 600 holdout tiles (the trainer's split, seed 42), submitted round-robin by 4 closed-loop clients. The gateway exported traces to Jaeger during these runs. Raw `spotterload` reports are the JSON files next to this one.

## Versions

| Version | Variant | Holdout accuracy | Stage at start |
|---|---|---|---|
| 3 | ONNX int8 | 0.9654 | production |
| 2 | ONNX fp32 | 0.9657 | validated → **shadow** |

## Shadow statistics and gates

After 819 shadowed jobs, the registry's statistics and `GET /v1/versions/{id}/gates` reported:

| Gate | Result | Detail |
|---|---|---|
| `accuracy` | pass | 0.9657 vs production 0.9654 (tolerance 0.01) |
| `shadow_agreement` | pass | 819 comparisons, 818 agreeing (99.9%; needs ≥ 200 and ≥ 95%) |
| `shadow_latency` | pass | mean shadow model time 4.66 ms (budget 25 ms) |
| `approval` | **fail** until approved | 0 approvals |

`POST :promote` before approval returned a failed-precondition error naming `approval`. After `POST :approve`, promotion succeeded.

## Promotion and rollback

Times are the registry's stage-change timestamps (one transaction swaps both versions, so both rows carry the same time) and the worker's log line for the reload.

| Operation | Committed | Worker loaded the new production version | Delay |
|---|---|---|---|
| Promote v2 (v3 → `retired`) | 15:53:11.055 | 15:53:11.287 | 232 ms |
| Roll back (v2 → `rolled_back`, v3 → `production`) | 15:53:30.305 | 15:53:30.762 | 457 ms |

The delay is bounded by the outbox relay's 500 ms polling interval plus the artifact download and checksum.

**Rollback under load:** the rollback ran in the middle of a 10 s load run. 496 jobs submitted, **0 failed, 0 lost**. Completion events by serving version: 196 on v2 before the worker reloaded, then 1 in-flight job on v2 and 299 on v3.

## A worker thread-pool problem found during this run

The first shadow load run delivered **7.5 jobs/s at a p50 of 429 ms**, against about 60 jobs/s measured earlier without a shadow. Every hop slowed together. Gateway spans at 4 clients showed the registry lookup at 38 ms, the NATS publish at 42 ms, and the object-store write at 141 ms. The worker used **16 cores to serve 10.8 jobs/s**.

Cause: ONNX Runtime and PyTorch default to one intra-op thread per core, and ONNX Runtime's threads busy-wait between runs. With two sessions loaded (production and shadow), the worker's spinning threads starved every other process on the machine. Fix: one intra-op thread per model and spinning off (`MODEL_THREADS`, default 1).

| Run (4 clients, 10 s) | Throughput | p50 | p95 | p99 | Worker CPU |
|---|---|---|---|---|---|
| Shadow loaded, default threads | 7.5 jobs/s | 429 ms | 1,153 ms | 1,394 ms | 16.1 cores (measured on a repeat run at 10.8 jobs/s) |
| Shadow loaded, 1 thread, no spinning | 43.1 jobs/s | 84 ms | 123 ms | 233 ms | 0.49 cores |
| No shadow, 1 thread, no spinning (2 runs) | 63.1 / 65.3 jobs/s | 62 / 61 ms | 73 / 68 ms | 83 / 73 ms | not measured |

With one thread the production model took 1.74 ms per job (433 jobs).

## The cost of shadowing

With the fix in place, shadowing measured 43.1 jobs/s against 63–65 without it (one run with shadow). The comparison runs in the worker's processing loop after the job is acknowledged, so each job's latency is unchanged until the comparison delays the next fetch: about 7 ms per job (fp32 classification plus the publish). Ways to recover the throughput while shadowing: add worker replicas, or move comparisons onto their own bounded queue.

## Still open

Even with contention gone, the gateway's `store image` span is about 46 ms (p50) for a 3 KB tile, while a direct `put_object` of 3 KB from Python to the same SeaweedFS takes 5.7 ms (median of 20). Disabling the AWS SDK's default request checksums made no difference. The cause is not yet known.
