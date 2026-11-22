# 0012 — Drift detection: PSI against a training-time reference, with KS reported

- **Date:** 2026-11-22
- **Status:** Accepted

## Context

Models in the field see inputs that drift from their training data (sensor changes, weather, compression). Labels are rarely available at serving time, so drift has to be detected from what the system does see: image statistics the worker already computes, and the model's confidence. The detector must be explainable to operators, cheap, and produce few false alarms.

## Options considered

**What to compare against**
1. **The first N events after deployment.** No preparation needed, but drift present from the start is never detected, and a restart redefines "normal".
2. **A reference computed at training time** from the validation split, stored with the model version.

**The statistic**
3. **PSI** (population stability index) over reference-decile bins. Widely used in model monitoring, symmetric, has conventional thresholds (0.1 small, 0.2+ significant), and its per-bin terms explain where the shift is.
4. **Two-sample KS test.** Distribution-free with a p-value, but at large sample sizes the p-value flags shifts too small to matter.
5. **A learned drift detector** (e.g. a classifier separating reference from live data). More sensitive to complex shifts; harder to explain and to set thresholds for.

## Decision

Options 2 and 3, with 4 reported alongside.

- Each version's `reference.json` holds per-image features for the validation split, computed exactly as the worker computes them.
- The monitor alerts on PSI (≥ 0.2 to enter drift, < 0.1 to leave it) over a sliding window of 200 events, evaluated every 50.
- The KS statistic and p-value are included in each drift event as supporting evidence, not as the trigger.

## Consequences

- **Good:** Drift is measured against what the model was trained on, from the first request, and survives restarts.
- **Good:** Thresholds and per-feature numbers are simple to explain in an event or a dashboard.
- **Cost:** Every version needs a reference; versions published before this decision need `spotter-reference` run once.
- **Cost:** Image statistics and confidence won't catch every semantic shift.
