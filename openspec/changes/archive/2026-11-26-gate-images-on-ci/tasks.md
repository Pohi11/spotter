# Tasks

## 1. Decision

- [x] 1.1 Decision record 0024.

## 2. Workflows

- [x] 2.1 `images.yml` as a reusable workflow with a `publish` input (`compose_test.go`: workflow definition).
- [x] 2.2 CI's `images` job, needing every other job (`compose_test.go`: gated on CI).

## 3. Verification

- [x] 3.1 A push to `main` runs the image builds after the checks, and they publish.

## Results

First push with the gate (`5ba4aa1`): one CI run and no separate image run. The eight checks finished at 15:57:00; the `images` job's three builds started at 15:57:01 and 15:57:02. Checks took 9 min 40 s, the images 10 min 56 s more (Go 10 min 55 s, dashboard 3 min 49 s, Python 34 s), 20 min 37 s in all, against about 11 min to publish before the gate. On GHCR, each image's `sha-<commit>` and `main` tags resolve to the same digest.
