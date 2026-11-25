# Spec Delta

## ADDED Requirements

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
When the gateway can't be reached, the demo page SHALL offer a replay of the recording committed in `content/`, labeled throughout as a recording with its date. The replay SHALL feed the recorded inputs through the same logic as a live run at their recorded pace, except that a gap longer than 20 seconds SHALL be shortened, saying how long it really took. A replay SHALL send nothing to the gateway.

#### Scenario: Static host
- **WHEN** the demo page opens and the gateway answers 502 through the proxy
- **THEN** the page says the platform isn't reachable and offers the recorded run, labeled with its date

#### Scenario: Shortened training wait
- **WHEN** the recording has a 7-minute gap while the new version trains
- **THEN** the replay shortens it and says that the wait took 7 minutes in the recording
