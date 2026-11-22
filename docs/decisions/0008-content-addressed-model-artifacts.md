# 0008 — Model artifacts are content-addressed, with a metadata file beside each

- **Date:** 2026-11-22
- **Status:** Accepted

## Context

Every model version points at an artifact in the object store (decision record [0003](0003-object-store-seaweedfs.md)) and records its SHA-256 (the `model-registry` spec). Workers download artifacts to serve them; a later change compares versions and rolls back between them. Two failure modes matter: an artifact that changes after it was registered (a worker would serve something nobody evaluated), and an artifact nobody can reproduce or explain later ("which data and settings produced this?").

## Options considered

1. **Keys by model and version number** (`eurosat-classifier/v3/model.pt`). Readable, but the number is assigned by the registry after upload, so the key isn't known until registration, and a re-upload can silently overwrite bytes under a registered key.
2. **Keys by SHA-256 of the bytes** (`eurosat-classifier/<sha256>/model.pt`). The key is known before registration, can't hold different bytes than it names, and uploading the same model twice is harmless.
3. **An experiment tracker** (MLflow, Weights & Biases) as the store of record. Rich tooling, but a second registry beside Spotter's own, and a heavy dependency for one person's project.

For metadata:

4. **Encode everything in registry metrics.** Metrics are numbers only; commit hashes, versions, and hyperparameters don't fit.
5. **A `metadata.json` next to the artifact,** under the same content-addressed prefix.

## Decision

Options 2 and 5.

- Artifacts live at `s3://models/<model-name>/<sha256>/model.pt`, and the registry records the same SHA-256.
- `metadata.json` beside it records the split fingerprint, seed, hyperparameters, class order, code commit, library versions, training time, and holdout metrics.
- Consumers verify the SHA-256 of downloaded bytes against the registry before loading a model.

## Consequences

- **Good:** A registered version can't silently change: different bytes would have a different key and fail checksum verification.
- **Good:** Publishing is idempotent, so a trainer that crashes between upload and registration can just run again.
- **Good:** Any version can be traced back to the exact data split and code that produced it.
- **Cost:** Keys aren't human-friendly; the registry is the index from version number to key.
- **Cost:** Orphaned artifacts (uploaded, never registered) accumulate; a cleanup job could delete keys no version references.
