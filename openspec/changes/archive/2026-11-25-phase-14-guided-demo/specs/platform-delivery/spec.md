# Spec Delta

## ADDED Requirements

### Requirement: The demo can retrain on drift without a terminal
`deploy/demo/compose.yaml` SHALL define a `retrainer` service, only in the `retrain` profile, running `spotter-retrain --watch` from the Python image with the registry, NATS, and object-store settings and the bootstrap's dataset volume. It SHALL publish no port and SHALL not be part of the demo without the profile.

#### Scenario: Rendered with the retrain profile
- **WHEN** the file is rendered with `--profile retrain`
- **THEN** it contains a `retrainer` service running `spotter-retrain --watch` that mounts the same dataset volume as `bootstrap` and publishes no port

#### Scenario: Rendered without it
- **WHEN** the file is rendered without the profile
- **THEN** it has no `retrainer` service
