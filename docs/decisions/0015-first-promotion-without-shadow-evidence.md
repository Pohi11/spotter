# 0015 — The first version of a model is promoted on approval alone

- **Date:** 2026-11-22
- **Status:** Accepted. Amends [0013](0013-gated-promotion-shadow-traffic-human-approval.md).

## Context

Decision record 0013 requires at least 200 shadow comparisons before promotion. The worker shadows only next to a production version: it needs production's answer to compare with, and it can't finish a job without one. Installing the platform into an empty cluster showed the consequence: a model's first version could never collect comparisons, so it could never be promoted, and the model could never serve.

## Options considered

1. **A separate bootstrap call** that puts a version into production without gates. Another way into production to secure and audit.
2. **Shadow without production:** let the worker run a lone shadow version and record nothing. There is still no answer to compare with, so agreement can't be measured.
3. **With no production version, the shadow gates pass and say why;** accuracy has nothing to be compared with either, so the human approval decides.

## Decision

Option 3. `shadow_agreement` and `shadow_latency` pass with "no production version to compare with" when the model has no production version, as `accuracy` already did. The approval gate is unchanged, so a person still decides, with the reason recorded.

## Consequences

- **Good:** A fresh install can reach a serving model through the same promotion path and audit trail as every later version.
- **Cost:** The first version reaches production without evidence from live traffic. Its holdout metrics and the approver's judgement are all there is, which the gate report states.
