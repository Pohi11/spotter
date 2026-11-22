# Failure lab: the bus against a direct call (2026-11-22)

`tools/failure-lab.sh` ran `spotterchaos` ten times: both modes under five conditions. Each run: 4 closed-loop clients for 40 s over 400 holdout tiles, the fault injected at 10 s and healed at 20 s, then up to 60 s for outstanding jobs. One laptop (Windows 11, 22 logical cores); NATS, Postgres, and SeaweedFS in Docker Desktop; registry and gateway on the host; the worker started by `spotterchaos` with one model thread and no shadow model (`SHADOW=off`), serving version 3 (ONNX int8). Raw reports: `2026-11-22-failure-lab-<mode>-<fault>.json`, each with a per-second timeline.

- **Bus (asynchronous):** `SubmitInference` returns a job id; the client waits for the job's `InferenceCompleted`, read from the stream.
- **Direct call (synchronous):** `ClassifySync`; the gateway calls the worker's gRPC classifier and answers in the same call.

Both check the model with the registry first.

## Results

| Condition | Mode | Requests | Rejected | Lost | Duplicates | p50 | p99 | Max | Completions per second before the fault |
|---|---|---|---|---|---|---|---|---|---|
| No fault | Bus | 2,430 | 0 | 0 | 0 | 64 ms | 94 ms | 180 ms | 61.9 |
| | Direct | 17,138 | 0 | 0 | 0 | 9 ms | 14 ms | 76 ms | 435.2 |
| Worker killed at 10 s, new one started at 20 s | Bus | 1,373 | **0** | 0 | 0 | 63 ms | 86 ms | 30,087 ms | 61.6 |
| | Direct | 11,598 | **1,204** | 0 | 0 | 9 ms | 14 ms | 84 ms | 428.7 |
| NATS restarted at 10 s | Bus | 2,071 | 2 | 0 | 0 | 62 ms | 86 ms | 5,172 ms | 62.1 |
| | Direct | 18,830 | 0 | 0 | 0 | 8 ms | 14 ms | 308 ms | 474.7 |
| Postgres paused 10 to 20 s | Bus | 1,912 | 0 | 0 | 0 | 62 ms | 83 ms | 9,912 ms | 63.2 |
| | Direct | 13,853 | 0 | 0 | 0 | 9 ms | 13 ms | 9,958 ms | 457.0 |
| Object store paused 10 to 20 s | Bus | 1,894 | 3 | 0 | 0 | 63 ms | 83 ms | 10,009 ms | 63.1 |
| | Direct | 18,753 | 0 | 0 | 0 | 8 ms | 13 ms | 79 ms | 466.0 |

Rejected: the client got an error and would have to retry. Lost: accepted but never completed. Duplicates: more than one stored completion for one job.

## What the timelines show

- **Worker killed.** Bus: no request failed; completions stopped from 10 s until the new worker was ready, and throughput reached 80% of its earlier rate 12 s after the new worker was started (the worker takes about 10 s to start and load its model). The job the killed worker held waited for its acknowledgement deadline (`ACK_WAIT`, 30 s) and was redelivered: that is the 30,087 ms maximum. Direct call: every request between the kill and the new worker being ready failed, 76 to 80 per second from 4 clients backing off 50 ms between tries, 1,204 in all; throughput recovered 7 s after the new worker started. The number of rejections depends on how fast clients retry; that every request in the outage failed does not.
- **NATS restarted.** Bus: completions stopped for about 4 s, then ran at about half the earlier rate for about 5 s before returning to normal; 2 submissions failed. The cause of the slow period was not investigated. Direct call: not on its path, unaffected. (A restart has no heal step, so the recovery figure in the JSON, measured from 20 s, says nothing for this fault.)
- **Postgres paused.** Both modes stalled for the 10 s pause: each request checks the model with the registry, whose queries waited for the database. Nothing failed or was lost in either mode; both recovered within a second of unpausing.
- **Object store paused.** Bus: submissions waited on the image write for the whole pause; 3 hit the clients' 10 s deadline and failed; nothing accepted was lost. Direct call: not on its path, unaffected.

## Summary

Across every fault the bus lost no accepted job and stored no duplicate completion. It turned a worker crash, which failed every request in the direct design, into a delay. It pays for that on every request: on this machine it ran at about a seventh of the direct call's throughput with about 55 ms more median latency, most of it writing the image to the object store (see the load tests), and it has two more dependencies on its path, each of which stalled it when paused or restarted.
