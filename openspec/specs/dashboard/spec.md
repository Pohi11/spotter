# dashboard Specification

## Purpose
Defines the operator dashboard: what it shows about the platform, what it lets a person do, and how it reaches the platform.

## Requirements

### Requirement: The browser reaches the gateway only through an allowlisted proxy
The dashboard SHALL forward browser requests under `/api/` to the gateway (`GATEWAY_URL`) only for these routes: reading a model, its versions, a version, its history, and its gates; approving and promoting a version; rolling back a model; submitting an inference and reading a job; and the event feed. Any other path or method SHALL be answered with 404 without calling the gateway. The gateway's address SHALL never be sent to the browser.

#### Scenario: Allowed route
- **WHEN** the browser sends `GET /api/v1/models/eurosat-classifier/versions`
- **THEN** the dashboard forwards it to the gateway and returns the gateway's status and body

#### Scenario: Route outside the list
- **WHEN** the browser sends `POST /api/v1/models` or `GET /api/openapi.json`
- **THEN** the response is 404 and the gateway receives no request

### Requirement: The home page explains the platform with one recorded image
The home page SHALL state what Spotter is and does in plain language and follow one EuroSAT tile through the platform with the values a recorded run produced: the tile, its predicted class and confidence, the model version that served it, and how long each hop took.

#### Scenario: Worked example
- **WHEN** a visitor opens the home page
- **THEN** it shows the recorded tile with its predicted class, confidence, serving version, and per-hop timings from the recorded run

### Requirement: A visitor can classify a sample tile
The home page SHALL offer EuroSAT holdout tiles with the drift experiment's degradations. Submitting one SHALL show, as they become known, the job id, its status, the predicted class and confidence, the runner-up classes, the serving version, and the true class. If the platform can't be reached, the panel SHALL say so and keep the choice for a retry.

#### Scenario: Successful classification
- **WHEN** a visitor picks the Forest sample without degradation and submits it
- **THEN** the panel shows the job id, then the status `SUCCEEDED`, the predicted class, its confidence, the serving version, and the true class

#### Scenario: Platform unreachable
- **WHEN** the gateway can't be reached
- **THEN** the panel shows that the platform is unreachable and the Submit button stays available

### Requirement: Demo samples are generated with the drift experiment's degradations
The sample tiles SHALL be exported by a script from the EuroSAT holdout split with the degradation functions of `spotter-degrade`, and a manifest SHALL list for each file its true class, degradation, and strength.

#### Scenario: Manifest matches files
- **WHEN** the export script runs
- **THEN** every manifest entry names a written file, and each degraded file equals the degradation function's output for its source tile

### Requirement: The live page shows events as they arrive
The live page SHALL connect to the event feed and show the newest events first with their stream, subject, type, time, and a one-line summary (for a completion: job, class, confidence, version; for a stage change: version and stages; for drift: version and drifting features), keep a count per stream, and show when the connection is lost and being retried.

#### Scenario: New event
- **WHEN** an `InferenceCompleted` event arrives
- **THEN** it appears first in the list with its summary, and the `INFERENCE` count goes up by one

#### Scenario: Connection lost
- **WHEN** the feed connection drops
- **THEN** the page says it is reconnecting until events flow again

### Requirement: The model page shows every version and its stage
The model page SHALL list the model's versions with their number, stage, framework, holdout accuracy, and when each was registered, show each version's stage history with the recorded reasons, and mark the production version.

#### Scenario: Versions listed
- **WHEN** the model has versions 1 to 3 with version 3 in production
- **THEN** the page lists all three with their stages and marks version 3 as production

### Requirement: Promotion is shown gate by gate and needs a person
For a version in `shadow`, the model page SHALL show each promotion gate with whether it passed and its reason, an approval form that records the name typed into it, and a Promote button that is enabled only when every gate passes. The page SHALL show the registry's answer to each action, including a refusal.

#### Scenario: Gate fails
- **WHEN** the shadow version's `shadow_agreement` gate fails
- **THEN** the gate is shown as failed with its reason and the Promote button is disabled

#### Scenario: Approve then promote
- **WHEN** every gate but `approval` passes, a person approves with a name, and then promotes
- **THEN** the approval gate passes, Promote is enabled, and after promotion the version is shown in production

### Requirement: Rollback names what it will restore and asks first
The model page SHALL offer a rollback only when a production version exists, name the version that will be restored (the last one retired from production) before acting, and require a confirmation.

#### Scenario: Confirm rollback
- **WHEN** a person starts a rollback while version 3 is in production and version 2 was last retired
- **THEN** the page names version 2 as the version to restore and calls the rollback API only after confirmation

### Requirement: The drift page charts PSI over time
The drift page SHALL chart each feature's PSI from `DriftEvaluated` events per version, with the detection (0.2) and resolution (0.1) thresholds drawn, and state whether each version is drifting.

#### Scenario: Drifting version
- **WHEN** the latest `DriftEvaluated` for version 3 says it is drifting with mean brightness PSI 0.8
- **THEN** the page marks version 3 as drifting and its chart shows the brightness series above the 0.2 line

### Requirement: Results come from dated reports
The results page SHALL show the measured results (model variants, load tests, drift detection, promotion and rollback timing, Kubernetes autoscaling) from the JSON files in `benchmarks/`, each with the report's date and how it was measured. No number on the page SHALL be typed by hand.

#### Scenario: Model variants table
- **WHEN** a visitor opens the results page
- **THEN** the model variants table shows each variant's size, holdout accuracy, and latency read from `benchmarks/2026-11-22-model-variants.json`, with the report date

### Requirement: Architecture diagrams have text equivalents
Each diagram on the architecture page SHALL have a title and a text description that screen readers announce, and SHALL name the code modules it shows. A diagram wider than the screen SHALL scroll inside its frame instead of the page.

#### Scenario: Diagram description
- **WHEN** a screen reader reaches a diagram
- **THEN** it announces the diagram's title and its description

### Requirement: Pages work by keyboard and at phone width
Every page SHALL be usable by keyboard with a visible focus indicator, and at a 375-pixel-wide viewport SHALL not scroll horizontally.

#### Scenario: Phone width
- **WHEN** any page is opened at 375 by 812 pixels
- **THEN** the page's width equals the viewport's

### Requirement: Scenario tiles are a fixed sample of the holdout split
`spotter-demo-samples` SHALL also write a scenario set: a seeded random sample of the EuroSAT holdout split (default 300 tiles, so the class mix follows the split's), with one bundle per variant (`clean`, and each degradation at full strength) listing every tile's id, true class, and JPEG bytes. Each degraded tile SHALL equal `degrade_jpeg`'s output for its source tile, and every bundle SHALL list the same tiles in the same order.

#### Scenario: Same tiles in every bundle
- **WHEN** the scenario set is exported
- **THEN** the clean bundle and each degradation's bundle list the same tile ids in the same order, each degraded tile equals the degradation function's output, and running the export again writes identical bundles

### Requirement: The demo page walks through the loop step by step
The demo page SHALL present the scenario as ordered steps: normal traffic, haze rolls in, retraining, shadow traffic, human approval, and the new version on hazed tiles. Each sending step SHALL say what it sends and what to watch for, and SHALL send its tiles through the inference API at a visible, adjustable pace with a progress count.

#### Scenario: Paced sending
- **WHEN** a sending step runs at 10 tiles a second
- **THEN** the page submits about 10 scenario tiles a second through the proxy and shows how many it has sent and how many have completed

### Requirement: Demo steps complete on the platform's evidence
Steps SHALL complete on the platform's evidence, not on a timer: the normal step on a not-drifting evaluation of the production version after 200 of its tiles completed, the haze step on a `DriftDetected` for the production version, the retraining step on a newer version reaching `shadow`, the shadow step when the agreement gate passes, and the approval step on that version's stage change to `production`.

#### Scenario: Drift ends the haze step
- **WHEN** the haze step is sending and a `DriftDetected` arrives for the production version
- **THEN** the page stops sending, marks the step done with the number of hazed tiles sent, and moves to the retraining step

#### Scenario: A new version reaches shadow
- **WHEN** the retraining step is waiting and a stage change to `shadow` arrives for a version newer than production
- **THEN** that version becomes the candidate and the shadow step is next

### Requirement: The demo page shows accuracy only for its own labeled tiles
The page SHALL show, for the tiles it sent, accuracy per step and serving version, labeled as known only because the demo knows each tile's true class. Completions of jobs it didn't send SHALL be shown but not counted.

#### Scenario: Accuracy by version
- **WHEN** a completed job for a scenario tile arrives
- **THEN** it counts towards the accuracy of the version that served it in the current step, right or wrong against the tile's true class

### Requirement: The demo page explains every event it sees
The demo page SHALL show the events of the scenario as they arrive, newest first, each with the component that published it, its subject, and one sentence on why it happened and what reacts to it. It SHALL show the production and candidate versions and, for the candidate, each promotion gate with its reason as reported by the registry.

#### Scenario: A drift event explained
- **WHEN** a `DriftDetected` arrives
- **THEN** the page names the monitor as its publisher, the subject `monitor.drift.detected.<model>`, the drifted features, and the retrainer as the component that consumes it

### Requirement: The demo page shows the arithmetic behind each drift evaluation
For the latest `DriftEvaluated` of the version under watch, the demo page SHALL show each feature's PSI against the 0.2 threshold and its reference and window means and, for a chosen feature, the reference share and window share in each of the 10 bins, each bin's contribution `(w − r) · ln(w / r)`, and their sum, which equals the published PSI.

#### Scenario: Contributions add up
- **WHEN** an evaluation's feature has shares for 10 bins
- **THEN** the page lists 10 contributions whose sum, shown to three decimals, equals the feature's PSI to three decimals

### Requirement: The demo page never approves
The demo page SHALL not approve or promote on its own. The approval step SHALL show the gates and a form where a person enters their name and reason, and promotion SHALL be a separate action that person takes. In a replay, the step SHALL show the recorded approval as a person's, with the name and reason as recorded.

#### Scenario: Approval waits for a person
- **WHEN** the scenario reaches the approval step with every other gate passed
- **THEN** no approval or promotion request is sent until a person submits the form, and promotion is sent only when that person asks for it

### Requirement: A live demo run can be saved as a recording
A live run of the demo page SHALL log every input it reacts to (events, tiles sent, version lists, gate reports, step changes) with its time since the run began, and SHALL let the visitor save that log as a JSON recording.

#### Scenario: Saved run
- **WHEN** a visitor saves a live run
- **THEN** the file lists every input in the order the page received it, each with its time, and replaying it reaches the same steps

### Requirement: The demo page replays a recording when the platform is unreachable
When the gateway can't be reached, the demo page SHALL offer a replay of the recording committed at `dashboard/public/recordings/demo.json`, labeled throughout as a recording with its date. The replay SHALL feed the recorded inputs through the same logic as a live run at their recorded pace, except that a gap longer than 20 seconds SHALL be shortened, saying how long it really took. A replay SHALL send nothing to the gateway.

#### Scenario: Static host
- **WHEN** the demo page opens and the gateway answers 502 through the proxy
- **THEN** the page says the platform isn't reachable and offers the recorded run, labeled with its date

#### Scenario: Shortened training wait
- **WHEN** the recording has a 7-minute gap while the new version trains
- **THEN** the replay shortens it and says that the wait took 7 minutes in the recording
