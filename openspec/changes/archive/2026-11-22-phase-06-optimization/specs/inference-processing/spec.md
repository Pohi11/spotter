# Spec Delta

## MODIFIED Requirements

### Requirement: The worker serves its model's production version
The worker SHALL load the version of its configured model that is in stage `production`, downloading its artifact from the object store and refusing to load it unless the artifact's SHA-256 equals the checksum registered for the version. Versions with framework `pytorch` SHALL be loaded with PyTorch and versions with framework `onnx` with ONNX Runtime; both SHALL produce results in the same form. While no production version is loaded, jobs SHALL be retried later rather than failed immediately.

#### Scenario: Production version loaded
- **WHEN** the worker starts and the model has a production version
- **THEN** it loads that version and reports its id and number with every result

#### Scenario: ONNX production version
- **WHEN** the production version has framework `onnx`
- **THEN** the worker serves it with ONNX Runtime and results report that version

#### Scenario: Checksum mismatch
- **WHEN** the stored artifact's SHA-256 differs from the registered checksum
- **THEN** the worker does not load it and processes no jobs with it

#### Scenario: No production version yet
- **WHEN** a job arrives and no production version is loaded
- **THEN** the job is negatively acknowledged with a delay and delivered again later
