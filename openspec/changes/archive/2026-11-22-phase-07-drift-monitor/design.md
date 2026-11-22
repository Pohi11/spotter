# Design

## Context

The worker already publishes per-image input statistics (RGB channel means and standard deviations) and the prediction with every `InferenceCompleted` (change `phase-04-event-driven-inference`). Detection choices are recorded in decision record [0012](../../../../docs/decisions/0012-drift-detection-psi-against-training-reference.md).

## Features

Per event: `mean_r`, `mean_g`, `mean_b` (channel means), `contrast` (the mean of the three channel standard deviations), and `confidence` (top-class probability). The first four describe the input; confidence describes the model's reaction to it.

## Decisions

### D1. The reference comes from training time
`reference.json` holds the five features for each of the 4,050 validation tiles, computed with the published model for confidence. The monitor fetches it from the object store next to the version's artifact (the registry gives the artifact URI). Comparing against training data rather than "the first traffic we saw" means drift that is present from the first request is still caught, and a restart doesn't reset what "normal" means.

### D2. The statistics are pure functions in the monitor's domain
`PSI(reference, window []float64)` and `KS(reference, window []float64) (d, p float64)` have no I/O and table tests with known values. Bins are reference deciles; shares are floored at 1e-4. The KS p-value uses the asymptotic Kolmogorov distribution with the effective sample size n·m/(n+m).

### D3. A detector per version with hysteresis
`Detector` holds a ring buffer of 200 events and a state (ok or drifting). Every 50th event it computes PSI and KS for each feature; ok → drifting when any PSI ≥ 0.2, drifting → ok when all PSI < 0.1. Only transitions produce events. The 0.1–0.2 band prevents flapping on borderline traffic.

### D4. The window lives in memory
The reference is durable (object store); the window is the last 200 events, refilled in seconds at normal traffic. Persisting it was rejected as complexity without benefit. Drift events carry a message id of `drift-<version>-<state>-<window end stream sequence>`, so a redelivered batch after a crash doesn't publish twice within the duplicate window.

### D5. Hexagonal like the other services
`domain` (features, PSI, KS, detector), `ports` (reference source, event publisher), `app` (observe an event, publish transitions), `adapters` (JetStream consumer `drift-monitor` on `inference.completed.>`, NATS publisher, registry + S3 reference source with a cache). An `archtest` guards the boundary.

### D6. Degradations are deterministic and graded
`spotter-degrade` applies one degradation at strength 0–1 with a fixed seed: darken (multiply by 1 − 0.5·s), blur (Gaussian, radius 2·s), noise (Gaussian, σ = 0.15·s), haze (blend toward light grey by 0.6·s). The experiment submits degraded holdout tiles and records how many events it took until `DriftDetected`, and runs clean traffic to count false alarms.

## Risks and trade-offs

- **Image statistics miss semantic drift** (new kinds of scenes with similar brightness). Confidence partly covers it; per-class monitoring is out of scope.
- **Thresholds are conventions** (PSI 0.2/0.1), tuned by the false-alarm experiment rather than learned.
