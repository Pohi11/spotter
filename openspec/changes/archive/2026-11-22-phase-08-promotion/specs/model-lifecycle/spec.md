# Spec Delta

## ADDED Requirements

### Requirement: Production is entered only through promotion or rollback
The generic transition operation SHALL refuse any transition into `production` with a failed-precondition error that names the promotion operation, and change nothing. Only gated promotion (from `shadow`) and rollback (from `retired`) SHALL move a version into `production`.

#### Scenario: Generic transition to production
- **WHEN** a client transitions a shadow version to `production` with the generic operation
- **THEN** the call fails with a failed-precondition error naming `PromoteVersion`, and the version stays in `shadow`
