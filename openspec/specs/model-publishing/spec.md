# model-publishing Specification

## Purpose
Defines how a trained model becomes a registered version: where its artifact is stored, what is recorded with it, how it is registered, and when it is validated.

## Requirements

### Requirement: Artifacts are content-addressed
A model artifact SHALL be stored at `s3://<bucket>/<model-name>/<sha256>/model.pt`, where `<sha256>` is the SHA-256 of the artifact's bytes. The checksum registered with the version SHALL equal the SHA-256 of the stored bytes. Publishing the same bytes twice SHALL produce the same location.

#### Scenario: Location and checksum agree
- **WHEN** a model is published
- **THEN** the object at the returned URI has a SHA-256 equal to the checksum in the URI and in the registered version

#### Scenario: Republishing is idempotent
- **WHEN** the same artifact bytes are published twice
- **THEN** both calls return the same URI

### Requirement: Every artifact carries reproducibility metadata
A `metadata.json` SHALL be stored next to each artifact, recording at least: the split fingerprint, the seed, the hyperparameters, the class names in output order, the code commit, the Python, PyTorch, and torchvision versions, the training duration, and the holdout metrics.

#### Scenario: Metadata is complete
- **WHEN** a model is published
- **THEN** its `metadata.json` contains every field listed above with a non-empty value

### Requirement: Trained versions are registered with holdout metrics
After publishing, the trainer SHALL register the version with the registry over gRPC, creating the model first if it does not exist. The registered metrics SHALL include holdout `accuracy`, `macro_f1`, and `recall_<class>` for every class.

#### Scenario: First version of a new model
- **WHEN** a model is trained for a model name the registry doesn't know
- **THEN** the model is created and version 1 is registered with the holdout metrics and the artifact's URI and checksum

#### Scenario: Model already exists
- **WHEN** a model is trained for a model name the registry already knows
- **THEN** a new version is registered under the existing model

### Requirement: Versions that meet the accuracy floor are validated
If holdout accuracy is at least the configured floor (default 0.85), the trainer SHALL move the new version from `registered` to `validated` with a reason stating the accuracy and the floor. Otherwise the version SHALL stay `registered` and the trainer SHALL report that it did not meet the floor.

#### Scenario: Above the floor
- **WHEN** a version's holdout accuracy is 0.93 and the floor is 0.85
- **THEN** the version is moved to `validated` with a reason that includes 0.93 and 0.85

#### Scenario: Below the floor
- **WHEN** a version's holdout accuracy is 0.80 and the floor is 0.85
- **THEN** the version stays `registered`

### Requirement: Each version carries a reference distribution
Each published version SHALL have a `reference.json` next to its artifact holding, for every validation-split image, the features the worker reports at serving time — mean and standard deviation of each RGB channel on a 0–1 scale — and the version's top-class confidence. The features SHALL be computed exactly as the worker computes them.

#### Scenario: Reference written with a version
- **WHEN** a version is published
- **THEN** `reference.json` next to its artifact holds one value per validation image for `mean_r`, `mean_g`, `mean_b`, `contrast`, and `confidence`

#### Scenario: Same features as serving
- **WHEN** the reference features of an image are compared with the input statistics the worker computes for the same JPEG
- **THEN** they agree to within 1e-6

### Requirement: A trained version can go to shadow on request
When `SHADOW_AFTER_VALIDATION` is `true` and the version was validated, `spotter-train` SHALL move it to `shadow` using its current revision, and SHALL request no other stage.

#### Scenario: Validated then shadow
- **WHEN** training validates a version with `SHADOW_AFTER_VALIDATION=true`
- **THEN** the transitions requested are to `validated` and then to `shadow`, the second with the revision read after validation
