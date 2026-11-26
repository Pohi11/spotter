# Proposal

## Why

The image workflow publishes on every push to `main` without waiting for CI, so a commit whose checks fail still gets a `:main` image (it happened at `d063a5a`). The public demo host deploys `:main`.

## What Changes

- **Gated publishing:** `images.yml` becomes a reusable workflow with a `publish` input. CI calls it in a final `images` job that needs every other CI job, so images are built only after all checks pass and pushed only for pushes to `main`.
- **Narrower token:** only the `images` job gets `packages: write`.
- Decision record 0024.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `platform-delivery`: images are published only after every CI check passes.

## Impact

- **Changed:** `.github/workflows/images.yml`, `.github/workflows/ci.yml`, `deploy/demo/compose_test.go`.
- **New:** decision record 0024.
- **Unchanged:** the images themselves, their tags, the demo, and the public host.
