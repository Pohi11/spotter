# End-to-end load tests, 2026-11-22

`spotterload` against the local stack (registry, gateway, inference worker on the host; NATS, SeaweedFS, Postgres in Docker Desktop on Windows), 200 holdout tiles submitted round-robin, 30 s per run. Latency is timed from just before `SubmitInference` to the job's `InferenceCompleted` event on NATS. Telemetry export was off. Raw reports are the JSON files next to this one.

| Run | Model in production | Workers | Mode | Completed | Lost | Jobs/s | p50 ms | p95 ms | p99 ms |
|---|---|---|---|---|---|---|---|---|---|
| open-loop-overload-pytorch-fp32 | v1 PyTorch fp32 | 1 | open, 8 clients | 3,854 | 694 | 64.2 | 19,249 | 33,101 | 34,290 |
| closed-loop-pytorch-fp32 | v1 PyTorch fp32 | 1 | closed, 4 clients | 1,751 | 0 | 58.3 | 67.3 | 80.3 | 99.1 |
| closed-loop-onnx-int8 | v3 ONNX int8 | 1 | closed, 4 clients | 1,797 | 0 | 59.8 | 64.0 | 86.9 | 112.9 |
| closed-loop-onnx-int8-two-workers-fetch8 | v3 ONNX int8 | 2 | closed, 4 clients | 960 | 0 | 31.2 | 88.8 | 275.9 | 973.3 |
| closed-loop-onnx-int8-two-workers-fetch1 | v3 ONNX int8 | 2 | closed, 4 clients | 1,340 | 0 | 44.6 | 84.7 | 119.0 | 169.2 |

## What the numbers say

- **Open loop overloads one worker.** Clients submitted about 150 jobs/s; one worker completed about 64/s, so the queue grew and latency reached tens of seconds. The 694 "lost" jobs were still queued at the 30 s cut-off and completed afterwards.
- **A 4.6× faster model barely moves end-to-end latency.** The worker's own metric put model time at about 2.9 ms per image (23.35 s over 8,096 jobs, across both versions) out of roughly 65 ms per job.
- **Where the time goes.** Jaeger spans over 17 submissions through the gateway (median): `SubmitInference` 65.9 ms, of which **storing the image in SeaweedFS 50.0 ms**, the registry lookup 6.5 ms, recording the job in KV 1.3 ms, publishing to JetStream 1.4 ms.
- **A second worker didn't add throughput,** because the object store is shared. With batched fetches (8 per pull) it also made the tail much worse (p99 973 ms); fetching one message at a time removed that tail (p99 169 ms). The mechanism behind the batched-fetch tail was not confirmed by an isolated test.
- **Next lever:** keep small images (EuroSAT tiles are about 3 KB) inline in the request instead of in the object store, and use the object store only above a size threshold.
