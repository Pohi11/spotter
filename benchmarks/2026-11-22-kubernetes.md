# Kubernetes: install from empty, first promotion, and autoscaling (2026-11-22)

A one-node k3d cluster (k3s v1.35.5) on the development laptop (Windows 11, 22 logical processors, Docker Desktop), with the chart installed by Tilt using `deploy/helm/values-dev.yaml`: KEDA 2.21.1, worker autoscaling between 1 and 4 replicas at a lag threshold of 20, and a 30 s scale-down window. Load came from `spotterload` on the host through `kubectl port-forward` to the gateway (gRPC) and NATS. The raw reports are the files next to this one.

## Install from an empty cluster

`tilt ci` on a freshly created cluster (no KEDA, no namespace): **7 min 29 s** until every resource was ready, with **0 container restarts**. That time includes building both images (Docker's layer cache was warm from earlier builds), pushing them to the cluster's local registry (the Python image is 1.9 GB, 809 MB of it CPU PyTorch), installing KEDA, and installing the chart with `--wait`.

`helm test` (the `spotterctl smoke` Pod) and an upgrade check then passed through the Go install tests: `TestFreshInstall` in 8.3 s, and `TestUpgradeKeepsCredentials` (generated Secret unchanged after `helm upgrade`) in 5.1 s.

## Problems the install found

| Problem | Symptom | Fix |
|---|---|---|
| The Go image's user was the name `nonroot` | Every Go pod stuck in `CreateContainerConfigError`: Kubernetes can't verify `runAsNonRoot` for a named user | `USER 65532:65532` |
| A model's first version could never be promoted | With no production version the worker can't shadow, so `shadow_agreement` (≥ 200 comparisons) could never pass | Shadow gates pass with a stated reason when there is no production version; approval still decides ([0015](../docs/decisions/0015-first-promotion-without-shadow-evidence.md)) |
| Tilt reported success before pods were ready | `helm upgrade` without `--wait` returns once manifests are applied | `--wait` on the chart's Tilt resource |
| Tilt's apply timed out | KEDA's install takes longer than Tilt's 30 s default | `update_settings(k8s_upsert_timeout_secs=900)` |
| KEDA's `pollingInterval` and `cooldownPeriod` did nothing | KEDA warned that both apply only when scaling to zero; above zero the HPA polls KEDA's metrics and its scale-down window decides | Rendered only for `minReplicas: 0`; the HPA scale-down window is a chart value |

## First promotion in the empty cluster

The int8 artifact (and its drift reference) was copied from the local object store into the cluster's, then registered through the gateway's REST API, moved to `shadow`, and promoted:

- Gates before approval: `accuracy`, `shadow_agreement`, and `shadow_latency` passed with "no production version to compare with"; `approval` failed. `POST :promote` returned 400 naming `approval`.
- After `POST :approve`, `POST :promote` returned 200. The worker loaded the version within a second (log line at 21:35:20).

## Load spike: autoscaling vs one worker

64 closed-loop clients for 90 s, 600 holdout tiles round-robin, against the int8 version.

| Run | Jobs | Failed / lost | Throughput | p50 | p95 | p99 | Max |
|---|---|---|---|---|---|---|---|
| KEDA autoscaling (1 → 3 workers) | 17,029 | 0 / 0 | **188.7 jobs/s** | 270 ms | 900 ms | 1,134 ms | 2,364 ms |
| One worker, autoscaling off | 7,403 | 0 / 0 | 78.3 jobs/s | 693 ms | 912 ms | 1,278 ms | 12,062 ms |

Worker replicas, sampled every 2 s ([samples](2026-11-22-k8s-spike-replicas.txt)):

| Time | Event |
|---|---|
| 16:37:14 | spike starts |
| 16:37:17 | first sample with desired replicas 3 (average lag 52 per replica) |
| 16:37:32 | 3 replicas ready: **18 s after the spike started** |
| 16:38:44 | load ends; lag is 0 by the next sample |
| 16:39:02 | back to 1 replica: **18 s after the load ended** |

It stopped at 3 replicas, not the maximum of 4: 64 clients keep about 60 jobs waiting, and 60 ÷ 20 per replica = 3. Average lag held at 18–21 per replica, the target.

The single-worker run's 12 s maximum is one outlier; its cause wasn't investigated.
