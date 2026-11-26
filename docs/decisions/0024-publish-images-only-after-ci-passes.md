# 0024 — Publish images only after every CI check passes

- **Date:** 2026-11-26
- **Status:** Accepted. Amends decision record 0020's publishing trigger.

## Context

Decision record 0020 publishes the three images to GitHub Container Registry on every push to `main`, from a workflow (`images.yml`) that runs beside CI (`ci.yml`) and does not wait for it. Commit `d063a5a` failed CI and still published `:main` and its SHA tag. The public demo host (decision record 0023) deploys `:main`, so a broken commit could reach it: the tag carries no signal that the code behind it was tested.

## Options considered

1. **Keep the workflows independent.** Fastest feedback (images finish about 11 minutes after a push, in parallel with CI), but `:main` can point at an image whose tests failed.
2. **Trigger the image workflow with `workflow_run` when CI completes successfully.** Gated, keeps the files apart, but the run starts from the default branch's workflow file with a different event: the checkout ref and the SHA tag have to be taken from the triggering run, a pull request's images can no longer be built from the pull request's own workflow, and the link between a commit's checks and its images is split across two runs.
3. **Make the image workflow reusable and call it from CI as a final job that needs every other job.** Gated by GitHub's own `needs` semantics (a failed or skipped dependency skips the job), one run per commit shows the checks and the images together, and the image workflow keeps its own file and its own permissions. Images start only after the slowest check (the kind job, about 8 minutes), so a push takes about 19 minutes to publish instead of 11.

## Decision

Option 3. `images.yml` runs only through `workflow_call`, with a `publish` input. CI calls it in an `images` job that needs every other CI job, passes `publish: true` only on pushes, and grants `packages: write` to that job alone. Pull requests still build both platforms, without logging in or pushing.

## Consequences

- **Good:** An image tag, `:main` included, exists only for a commit whose checks all passed, so the demo host never pulls an untested build. The rest of the CI jobs no longer hold `packages: write`.
- **Cost:** Publishing waits for CI: about 8 extra minutes per push to `main`, and a pull request's image build also waits for its checks. A flaky check now blocks publishing until it is rerun.
- **Unchanged:** Deployment to the host stays a person's step (`deploy.sh`); this changes which images can exist, not when they are deployed.
