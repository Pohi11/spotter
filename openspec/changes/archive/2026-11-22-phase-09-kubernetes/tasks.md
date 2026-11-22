# Tasks

## 1. Decision and images

- [x] 1.1 Decision record 0014. Go and Python Dockerfiles and `.dockerignore`; both images build.

## 2. Chart

- [x] 2.1 Chart with all workloads, vendored NATS, generated Secret, probes, resources, shutdown settings, KEDA ScaledObject, trainer Job; chart tests for each render-time scenario.

## 3. Smoke test and local loop

- [x] 3.1 `spotterctl smoke` and the Helm test Pod. k3d cluster file and Tiltfile; `tilt up` from an empty cluster.

## 4. First promotion

- [x] 4.1 Decision record 0015; shadow gates pass without a production version; tests.

## 5. CI and real run

- [x] 5.1 CI job: kind, KEDA, chart install, `helm test`, upgrade check, report into spec coverage.
- [x] 5.2 Measure a load spike scaling workers up and back down.
