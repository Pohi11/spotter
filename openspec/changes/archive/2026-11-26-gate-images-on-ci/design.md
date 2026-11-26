# Design

## Context

`images.yml` builds the Go, Python, and dashboard images for amd64 and arm64 and pushes them on pushes to `main` (decision record 0020). It runs beside CI and does not wait for it. The choice to gate it on CI, and how, is recorded in decision record [0024](../../../../docs/decisions/0024-publish-images-only-after-ci-passes.md).

## Decisions

### D1. A reusable workflow called from CI
`images.yml` has only a `workflow_call` trigger, so it never runs on its own and cannot bypass the checks. CI's `images` job calls it with `uses: ./.github/workflows/images.yml`. Inside a called workflow, `github.sha` and `github.ref` are the caller's, so the SHA and `main` tags are unchanged.

### D2. `needs` lists every other job, with no `if`
The `images` job needs `proto`, `go`, `python`, `dashboard`, `end-to-end`, `kubernetes`, `spec-coverage`, and `secrets`. Without an `if:` override, GitHub skips a job when any dependency fails, is skipped, or is cancelled. A test fails if a CI job is added without being listed, or if the job gains an `if:` that could run it after a failure.

### D3. An explicit `publish` input
CI passes `publish: ${{ github.event_name == 'push' }}`; the login step and the build's `push` depend on `inputs.publish`, not on the event name, so the called workflow states what it does and a test can read it.

### D4. Permissions on the calling job
The workflow's top-level permissions stay `contents: read`. The `images` job grants `contents: read` and `packages: write`, which the called workflow's token inherits; no other job can write packages.
