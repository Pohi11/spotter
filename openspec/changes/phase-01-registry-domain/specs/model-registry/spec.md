# Spec Delta

## Purpose

Defines models and model versions: what makes a version valid, how versions are numbered, and how they are looked up.

## ADDED Requirements

### Requirement: Model names are unique and well-formed
A model name SHALL be 3 to 63 characters, start with a lowercase letter, and contain only lowercase letters, digits, and hyphens. No two models SHALL share a name.

#### Scenario: Valid name is accepted
- **WHEN** a model named `eurosat-classifier` is registered
- **THEN** the model is stored and can be found by that name

#### Scenario: Malformed name is rejected
- **WHEN** a model named `EuroSAT Classifier` is registered
- **THEN** registration fails with a validation error naming the `name` field
- **AND** no model is stored

#### Scenario: Duplicate name is rejected
- **WHEN** a model named `eurosat-classifier` is registered a second time
- **THEN** registration fails with an already-exists error

### Requirement: A version records a verifiable artifact
A model version SHALL record an artifact URI of the form `s3://<bucket>/<key>`, the artifact's SHA-256 checksum as 64 lowercase hexadecimal characters, a framework of `pytorch` or `onnx`, and zero or more named numeric metrics with finite values. A new version SHALL start in stage `registered` with revision 1.

#### Scenario: Valid version is registered
- **WHEN** a version is registered with artifact `s3://models/eurosat/v1.pt`, a valid checksum, framework `pytorch`, and metric `accuracy` = 0.91
- **THEN** the version is stored in stage `registered` with revision 1 and the given metric

#### Scenario: Malformed checksum is rejected
- **WHEN** a version is registered with checksum `abc123`
- **THEN** registration fails with a validation error naming the `artifact_sha256` field
- **AND** no version is stored

#### Scenario: Unsupported framework is rejected
- **WHEN** a version is registered with framework `tensorflow`
- **THEN** registration fails with a validation error naming the `framework` field

#### Scenario: Non-S3 artifact URI is rejected
- **WHEN** a version is registered with artifact `/tmp/model.pt`
- **THEN** registration fails with a validation error naming the `artifact_uri` field

### Requirement: Version numbers are sequential per model
Each model's versions SHALL be numbered 1, 2, 3, … in registration order, with no gaps or duplicates, even when versions are registered concurrently. Numbering SHALL be independent for each model.

#### Scenario: Versions are numbered in order
- **WHEN** two versions of one model are registered one after the other
- **THEN** they are numbered 1 and 2

#### Scenario: Concurrent registrations get distinct numbers
- **WHEN** 10 versions of one model are registered concurrently
- **THEN** they are numbered exactly 1 through 10

#### Scenario: Each model has its own numbering
- **WHEN** one version is registered for each of two models
- **THEN** both versions are numbered 1

### Requirement: Unknown models and versions are reported as not found
Operations on a model or version that does not exist SHALL fail with a not-found error and change nothing.

#### Scenario: Version for an unknown model
- **WHEN** a version is registered for a model name that does not exist
- **THEN** registration fails with a not-found error

#### Scenario: Unknown version id
- **WHEN** a version is requested by an id that does not exist
- **THEN** the request fails with a not-found error

### Requirement: Versions are listed newest first
Listing a model's versions SHALL return all of them ordered by version number, highest first.

#### Scenario: Three versions are listed
- **WHEN** versions 1, 2, and 3 of a model exist and the model's versions are listed
- **THEN** the list is versions 3, 2, 1
