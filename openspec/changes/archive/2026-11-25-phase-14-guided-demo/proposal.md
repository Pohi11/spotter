# Proposal

## Why

The dashboard shows each part of the platform on its own page, but nothing shows the loop the platform exists for, end to end, while it happens: traffic shifts, the monitor notices, a new version is trained and shadowed, gates fill up, a person approves, and the new version serves. Showing it today takes a terminal, a load generator, and someone explaining. The drift monitor also publishes only its final PSI per feature, so a page can say *that* a feature drifted but not *why the number is what it is*.

## What Changes

- **Guided demo page** (`/demo`): a scenario in steps (normal traffic, haze rolls in, drift detected, retraining, shadow traffic, human approval, the new version on hazed tiles). Each step sends real tiles through the public inference API, follows the event feed, and explains in plain language which component acted, on which subject, and why. Because the page knows each tile's true class, it also shows accuracy, labeled as something production can't measure.
- **The arithmetic behind each drift evaluation:** `FeatureDrift` gains the bin edges and the reference and window shares behind its PSI (additive protobuf fields). The demo page draws reference against window per feature and lists each bin's contribution to the sum.
- **Scenario tiles:** `spotter-demo-samples` also writes a fixed, seeded set of holdout tiles (the split's class mix), clean and with each degradation at full strength, as one bundle per variant.
- **Recorded replay:** when the platform can't be reached (a static host, or the stack is down), the page replays a recorded run of the same scenario, labeled as a recording with its date. A recording is the log of a live run of the same page, so the replay runs the same code.
- **Compose demo:** an optional `retrain` profile runs `spotter-retrain --watch`, so the demo closes the loop without a terminal.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `dashboard`: the demo page, its drift arithmetic panel, scenario tiles, and recorded replay.
- `drift-monitoring`: comparisons carry the bins behind each PSI.
- `platform-delivery`: the Compose demo's optional retrainer.

## Impact

- **Protobuf:** three fields added to `FeatureDrift`; `buf breaking` passes (additive).
- **Monitor:** the domain returns bins with each result; detection, thresholds, and events are otherwise unchanged.
- **Dashboard:** a new page, a recording in `public/recordings/`, scenario bundles in `public/scenario/`.
- **Decision record:** 0022 (a guided demo driven from the browser through the public API, with a labeled recorded fallback).
- **Out of scope:** automating the approval in any mode, and any demo-only endpoint in the services.
