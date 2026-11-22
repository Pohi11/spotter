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
