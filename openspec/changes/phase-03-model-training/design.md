# Design

## Context

The registry and its API exist (changes `phase-01-registry-domain` and `phase-02-registry-api`). This change produces the first real model and the tooling to produce later ones. Everything runs on a laptop CPU.

## Layout

```
workers/model/src/spotter_model/      CLASSES, EuroSATNet, preprocess(): shared by trainer and (later) inference worker
workers/trainer/src/spotter_trainer/
  splits.py                           deterministic stratified split + fingerprint
  data.py                             download/verify EuroSAT, load images into memory
  metrics.py                          confusion matrix, accuracy, precision, recall, macro F1 (numpy, pure)
  train.py                            training loop (train + validation only), holdout evaluation
  publish.py                          content-addressed upload, metadata.json
  registry.py                         gRPC client: register model/version, validate
  cli.py                              `spotter-train`: wires it together from environment variables
conftest.py                           pytest `spec` marker → JUnit properties
```

## Decisions

### D1. A small CNN trained from scratch
Three convolution blocks (32, 64, 128 channels, each conv-BN-ReLU-pool) and a small classifier head, about 0.4 M parameters, for 64×64 RGB tiles. It trains on a CPU in minutes and needs no pretrained-weight download. A fine-tuned ResNet-18 would likely score a few points higher; the point here is a real, honestly measured model that later changes optimize, monitor, and replace.

### D2. Images are decoded once into memory
27,000 tiles × 3 × 64 × 64 bytes is about 330 MB as `uint8`. The trainer decodes every JPEG once into one tensor and normalizes per batch. This removes DataLoader worker processes, which are slow to start on Windows, and makes epochs CPU-bound on the model rather than on JPEG decoding.

### D3. Splits come from sorted paths and a seeded generator
`split(paths_by_class, seed)` sorts each class's relative paths, shuffles them with `random.Random(seed)`, and cuts at 70% and 85%. The manifest (split name → sorted paths) is serialized as canonical JSON and hashed; the hash is the split fingerprint recorded in `metadata.json`.

### D4. The holdout set is behind a separate call
`train(model, train_set, val_set, …)` never receives the holdout set. `evaluate(model, holdout_set)` runs after `train` returns. The best epoch is chosen on validation accuracy. A test passes an instrumented holdout set that records reads, and checks none happen before training ends.

### D5. Fixed normalization constants
Per-channel mean and standard deviation were measured once on the training split (seed 42) and fixed in `spotter_model`. Serving uses the same constants without needing the training data.

### D6. Content-addressed publishing
See decision record [0008](../../../docs/decisions/0008-content-addressed-model-artifacts.md). The trainer serializes the `state_dict` with `torch.save`, hashes the bytes, and uploads `model.pt` and `metadata.json` under `<model-name>/<sha256>/`. The bucket is created if missing.

### D7. Registration and the validation gate are plain code
The trainer calls `RegisterModel` (ignoring `ALREADY_EXISTS`), `RegisterVersion` with the holdout metrics, and, if `accuracy >= MIN_ACCURACY`, `TransitionVersion` to `validated` with revision 1. The floor is configuration, never a model output.

### D8. Python tests name their requirements
`@pytest.mark.spec("<capability>", "<requirement>")`, copied into JUnit XML properties by a root `conftest.py` hook (as decision record [0001](../../../docs/decisions/0001-adopt-openspec-with-cross-language-traceability.md) planned). `speccheck` gains a `-junit` flag, and CI runs it in a separate job after the Go and Python jobs upload their reports.

## Risks and trade-offs

- **CPU training limits the model size.** Accepted; a later change measures optimization on the same model.
- **The registry client and the S3 upload are tested against fakes and a local SeaweedFS, not the Go registry,** in Python's CI job. The end-to-end run (trainer → SeaweedFS → registry) is verified by hand and recorded.
