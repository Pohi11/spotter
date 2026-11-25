# Design

## Context

Every part of the loop exists: the inference API, the event feed (`GET /v1/events`, change `phase-10-dashboard`), the monitor's `DriftEvaluated` at every evaluation, the retrainer's watch mode (change `retrain-on-drift`), shadow statistics and gates (change `phase-08-promotion`). What's missing is a way to run the loop on demand and see each step's evidence without a terminal. The approach is recorded in decision record [0022](../../../docs/decisions/0022-guided-demo-driven-through-the-public-api.md).

## Decisions

### D1. The browser drives the scenario through the public API
The demo page submits tiles with `POST /v1/models/{name}/inferences` through the existing allowlisted proxy, exactly as any client would, and learns everything else from the event feed and the registry's read endpoints. No service gains a demo-only endpoint, and the proxy's allowlist doesn't change. The cost is that the tab has to stay open while a step sends.

### D2. Scenario tiles are bundled per variant
`spotter-demo-samples` writes `dashboard/public/scenario/<variant>.json` for `clean`, `haze`, `darken`, `noise`, and `blur` (full strength): the same 300 holdout tiles in the same order, each with its id, true class, and base64 JPEG. 300 random holdout tiles keep the split's class mix, which matters: the drift experiment found that a class-mix shift alone registers as drift. One file per variant means one request per step instead of hundreds, and a page that never sends a degradation never downloads it.

### D3. The monitor publishes the bins behind each PSI
`domain.Compare(reference, window)` returns the 9 decile edges and the 10 reference and window shares (floored at `MinShare`, exactly as summed), and `PSI` is computed from it, so the published numbers can't disagree with the statistic. `FeatureDrift` gains `bin_edges` (7), `reference_shares` (8), and `window_shares` (9); the change is additive, so `buf breaking` passes and old readers ignore them. About 240 bytes per feature, at most one evaluation per 50 completions.

### D4. One reducer, two sources
`lib/scenario.ts` is a pure reducer from inputs to the scenario state: `event` (a feed event), `sent` (a tile submitted, with its job id, tile, and variant), `versions` (the registry's version list), `gates` (a candidate's gate report), and `step` (a step started). Steps advance only on evidence in that state (spec: "The demo page walks through the loop step by step"). Live mode produces inputs from the sender, the event feed, and polling; replay mode produces them from a recording. Tests drive the reducer directly.

The page counts only its own jobs: it remembers the job id of every tile it sends, and a completion is matched by job id. Accuracy is the share of its own completions whose label matches the tile's true class, per step and per serving version.

### D5. Steps and their evidence

| Step | Sends | Done when |
|---|---|---|
| 1. Normal traffic | clean tiles | an evaluation of the production version arrives, after 200 of this step's tiles have completed, that says not drifting |
| 2. Haze rolls in | hazed tiles | `DriftDetected` for the production version |
| 3. Retraining | nothing | a stage change to `shadow` for a version newer than production (or the visitor picks one already in shadow, shown as trained earlier) |
| 4. Shadow traffic | clean tiles | the candidate's `shadow_agreement` gate passes |
| 5. A person decides | nothing | the candidate's stage changes to `production` |
| 6. Haze again | 100 hazed tiles | all of them completed |

Step 1 waits for "not drifting" so a monitor still drifting from earlier traffic resolves first; otherwise step 2's `DriftDetected` would never come, because drift is announced once per change. Step 4 sends clean tiles: shadow agreement is a regression check against what production already does well, and on hazed tiles a better candidate would disagree with production by design. Step 6 sends fewer than 200 tiles, so the new version's window never fills and no evaluation runs; the page explains that hazed inputs would drift against the new version's reference too, because the inputs really did change.

### D6. Pacing
The sender submits at a chosen rate (2, 5, 10, or 20 tiles a second; default 10) with at most 8 requests in flight, cycling through the bundle when a step needs more than 300 tiles. At 10 a second, step 1 takes about 25 seconds and step 2 typically 10 to 20, slow enough to watch the evaluations arrive every 50 tiles.

### D7. Transparency panels
- **What just happened:** each event with its publisher, subject, and one sentence (from `lib/explain.ts`) on why it was published and what consumes it. Job events are compact; registry and monitor events stand out.
- **Path strip:** gateway, JetStream, worker, monitor, retrainer, registry, each lit by the events it publishes, with counts.
- **Drift arithmetic:** for the latest evaluation of the watched version, PSI per feature against 0.2, both means, and for the chosen feature the reference and window share per bin, each contribution, and the sum.
- **Accuracy:** per step and serving version, labeled as known only because the demo has labels.
- **Gates:** the candidate's gate report, polled every 2 seconds in steps 4 and 5.

### D7b. The approval stays a person's
Step 5 reuses the model page's promotion form: name and reason, then a separate promote action. Nothing in the page or its tests approves on a timer or by default. A replay shows the recorded stage change and approval as recorded.

### D8. Recording and replay
A live run appends every reducer input with its time since the run began, and the page can save the log as JSON. `public/recordings/demo.json` is one such log from a run on the live stack. Replay feeds the log through the same reducer at the recorded pace; a gap longer than 20 seconds is cut to 3 seconds with a note of its real length (the training wait). The page checks the gateway once on load; on 502 it offers the replay instead of the live run. A replay never calls the proxy.

### D9. Compose demo retrainer
A `retrainer` service in the `retrain` profile runs `spotter-retrain --watch` with the bootstrap's dataset volume (the bootstrap downloaded EuroSAT into it), so `--profile retrain up -d` closes the loop.

## Risks and trade-offs

- **The page shares the platform with other traffic.** Completions it didn't submit are shown but not counted, yet they still enter the monitor's window; step 1's rule then waits longer, and the page says what it is waiting for.
- **Bundles add about 1 MB per variant** to the static site, fetched only for the variants a step uses.
- **A recording ages.** It shows the platform as it was on its date; the page says so rather than presenting it as live.
