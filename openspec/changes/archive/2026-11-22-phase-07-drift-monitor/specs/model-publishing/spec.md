# Spec Delta

## ADDED Requirements

### Requirement: Each version carries a reference distribution
Each published version SHALL have a `reference.json` next to its artifact holding, for every validation-split image, the features the worker reports at serving time — mean and standard deviation of each RGB channel on a 0–1 scale — and the version's top-class confidence. The features SHALL be computed exactly as the worker computes them.

#### Scenario: Reference written with a version
- **WHEN** a version is published
- **THEN** `reference.json` next to its artifact holds one value per validation image for `mean_r`, `mean_g`, `mean_b`, `contrast`, and `confidence`

#### Scenario: Same features as serving
- **WHEN** the reference features of an image are compared with the input statistics the worker computes for the same JPEG
- **THEN** they agree to within 1e-6
