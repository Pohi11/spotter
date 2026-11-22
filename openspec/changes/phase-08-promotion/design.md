# Design

## Context

Shadow is already a lifecycle stage (change `phase-01-registry-domain`) but nothing ran shadow versions. The promotion rules are recorded in decision record [0013](../../../docs/decisions/0013-gated-promotion-shadow-traffic-human-approval.md).

## Decisions

### D1. Gates are a pure function
`domain.EvaluateGates(candidate, production *ModelVersion, stats ShadowStats, approvals int, cfg GateConfig) []Gate` returns each gate with `Passed` and a human-readable `Detail`. The promotion use case refuses unless all pass; `GetPromotionGates` returns the same report without changing anything, so operators see exactly what promotion will check.

### D2. One repository method swaps production
`SwapProduction(ctx, out, outExpected, in, inExpected, changes, records)` moves one version out of production and one in, checking both expected revisions, in one transaction. Promotion and rollback both use it; the partial unique index (one production per model) still guards the database. Under READ COMMITTED the out-update runs first, so the index never sees two production rows mid-transaction.

### D3. Rollback target comes from history
The version to restore is the one whose most recent history entry is `production → retired` with the latest time. Versions retired from production by an operator for other reasons are candidates too; that is intended (rollback restores "what was serving before").

### D4. Shadow statistics are rows keyed by job
`shadow_comparisons(version_id, job_id, agree, shadow_ms, PRIMARY KEY (version_id, job_id))` with `INSERT … ON CONFLICT DO NOTHING`, so a redelivered event is counted once. Statistics are an aggregate query. A durable JetStream consumer (`registry-shadow-stats`, filter `inference.shadow.>`) feeds it.

### D5. The generic transition refuses production
`app.Service.TransitionVersion` returns `domain.ErrPromotionRequired` (mapped to `FAILED_PRECONDITION`) for `to == production`. The domain table still lists `shadow → production` and `retired → production`, because promotion and rollback use those edges. The contract suite reaches production through the repository directly where it tests database rules.

### D6. The worker treats shadow as best effort
The worker's model holder keeps `production` and `shadow` slots. After the production result is recorded and published, the worker runs the shadow model on the same preprocessed input and publishes `ShadowCompared`; any exception there is logged and ignored. Reload triggers now include stage changes into or out of `shadow`.

### D7. Approval is data
`approvals(version_id, approver, reason, approved_at)`. There is no identity system yet, so the approver is a free-text name recorded with the reason; authentication is out of scope and noted as a risk.

## Risks and trade-offs

- **Approver identity isn't verified.** Anyone who can reach the API can approve. Needs authentication before real use.
- **Shadow doubles model work** on the worker. Acceptable here (the model is about 3 ms of a 65 ms job, change `phase-06-optimization`).
