# Spec Delta

## Purpose

Lets a fresh installation's first trained version reach shadow without hand-written API calls; promotion still needs the gates and a person.

## ADDED Requirements

### Requirement: A trained version can go to shadow on request
When `SHADOW_AFTER_VALIDATION` is `true` and the version was validated, `spotter-train` SHALL move it to `shadow` using its current revision, and SHALL request no other stage.

#### Scenario: Validated then shadow
- **WHEN** training validates a version with `SHADOW_AFTER_VALIDATION=true`
- **THEN** the transitions requested are to `validated` and then to `shadow`, the second with the revision read after validation
