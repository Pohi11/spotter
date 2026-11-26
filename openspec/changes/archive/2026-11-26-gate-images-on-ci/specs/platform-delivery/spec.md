# Spec Delta

## MODIFIED Requirements

### Requirement: Images are built on every change and published from main
The image workflow SHALL build the Go, Python, and dashboard images for `linux/amd64` and `linux/arm64` on every pull request and every push to `main`, only after every other CI job has passed, and SHALL push them to `ghcr.io/<owner>/spotter-go`, `spotter-python`, and `spotter-dashboard` only on pushes to `main`, tagged with the full commit SHA and `main`. The image workflow SHALL not run except from CI.

#### Scenario: Workflow definition
- **WHEN** the workflow file is read
- **THEN** it builds all three images for both platforms, logs in and pushes only when asked to publish, and tags with the commit SHA and `main`

#### Scenario: Gated on CI
- **WHEN** the CI workflow is read
- **THEN** its `images` job calls the image workflow, needs every other CI job with no condition that could run it after a failure, and asks to publish only for push events, and the image workflow has no trigger other than being called
