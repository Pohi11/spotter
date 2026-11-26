# Tasks

## 1. Decision

- [x] 1.1 Decision record 0024.

## 2. Workflows

- [x] 2.1 `images.yml` as a reusable workflow with a `publish` input (`compose_test.go`: workflow definition).
- [x] 2.2 CI's `images` job, needing every other job (`compose_test.go`: gated on CI).

## 3. Verification

- [ ] 3.1 A push to `main` runs the image builds after the checks, and they publish.
