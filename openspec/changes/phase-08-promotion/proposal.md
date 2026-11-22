# Proposal

## Why

Today any caller can move a version into production with a generic transition, and swapping versions takes two calls with a gap between them where no model is in production (seen while promoting the int8 variant). A national-security platform needs promotion to be earned and auditable: a candidate must prove itself on real traffic without affecting answers, pass checks that are code, and be approved by a named person — and a bad promotion must be undoable in one step.

## What Changes

- The worker runs the newest `shadow` version next to production on every job, returns only production's result, and publishes a `spotter.v1.ShadowCompared` event (both predictions, agreement, both latencies).
- The registry consumes those events and keeps per-version shadow statistics.
- Add promotion gates, evaluated as code: holdout accuracy within tolerance of production, shadow agreement over enough comparisons, shadow latency within budget, and a recorded human approval.
- Add `ApproveVersion`, `GetPromotionGates`, `PromoteVersion`, and `RollbackModel` to the registry API (gRPC and REST). Promotion retires the current production version and promotes the candidate in one transaction; rollback marks production `rolled_back` and restores the previous production version in one transaction.
- **BREAKING:** the generic `TransitionVersion` no longer moves a version into `production`; only promotion and rollback do.

## Capabilities

### New Capabilities
- `model-promotion`: Shadow statistics, gates, approvals, atomic promotion, and rollback.
- `shadow-inference`: How the worker runs a shadow version and reports comparisons.

### Modified Capabilities
- `model-lifecycle`: production is entered only through promotion or rollback.

## Impact

- **Proto:** new registry RPCs and messages; `ShadowCompared` event.
- **Registry:** migration for approvals and shadow comparisons; new use cases, repository methods (both adapters, contract tests), a JetStream consumer for shadow events.
- **Worker:** loads production and shadow versions; publishes comparisons.
- **Out of scope:** canary traffic splitting, automatic promotion, multi-person approval.
