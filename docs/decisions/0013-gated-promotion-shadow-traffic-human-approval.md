# 0013 — Promotion needs shadow evidence, passing gates, and a human; production swaps atomically

- **Date:** 2026-11-22
- **Status:** Accepted

## Context

Until now the generic transition API could put any validated-and-shadowed version into production, and replacing production took two calls with a window in which no version was in production (observed while promoting the int8 variant). Model output must never promote anything; promotion needs evidence and an accountable decision, and must be reversible quickly.

## Options considered

1. **Promote on offline metrics alone.** Simple, but holdout accuracy says nothing about behaviour on today's traffic or on the serving path.
2. **Canary** (send a share of real traffic to the candidate). Real feedback, but some users get the candidate's answers before it's trusted.
3. **Shadow** (candidate sees every request, its answers are only recorded). No user sees the candidate's output; agreement with production and serving latency are measured on real traffic.

For the decision:

4. **Automatic promotion when metrics pass.** Fast; no accountable human.
5. **Gates as code plus a recorded human approval.**

## Decision

Options 3 and 5.

- The worker runs the newest shadow version on every job and publishes a comparison; the registry keeps per-version statistics.
- Gates: holdout accuracy within 0.01 of production, at least 200 shadow comparisons with ≥ 95% agreement, mean shadow latency ≤ 25 ms, and at least one recorded approval.
- Only `PromoteVersion` (gated) and `RollbackModel` move versions into production, each swapping production atomically in one transaction.

## Consequences

- **Good:** A promotion is explained by data (gate report), a person (approval), and a timeline (history and events).
- **Good:** No gap with zero production versions; rollback is one call.
- **Cost:** Shadow doubles model work on the worker while a shadow version exists.
- **Cost:** The approver isn't authenticated yet; an identity system is needed before any shared deployment.
