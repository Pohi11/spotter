# Proposal

## Why

The registry can store model versions, but there is no model yet. Everything after this (serving, optimization, drift monitoring, shadow testing) needs a real trained classifier with honestly measured accuracy, an artifact that can be verified byte for byte, and enough recorded context to reproduce it. Training also has to be disciplined from the start: a holdout set that training never touches, and metrics that report per-class weaknesses rather than one flattering number.

## What Changes

- Add a shared Python package with the EuroSAT classifier architecture, the class list, and the image preprocessing, so training and serving can never disagree on them.
- Add the trainer: download and verify EuroSAT, split it deterministically and stratified by class into train, validation, and holdout sets, train a small CNN on the CPU, and evaluate it on the holdout set once training ends.
- Add evaluation metrics: accuracy, per-class precision and recall, macro F1, and the confusion matrix.
- Publish each trained model to the object store under a key derived from its SHA-256, with a metadata file recording the data split hash, seed, hyperparameters, code commit, library versions, and metrics.
- Register the version with the registry over gRPC with its holdout metrics, creating the model if it doesn't exist, and move it to `validated` only when holdout accuracy meets a configured floor.
- Link Python tests to requirements and extend the spec coverage check to read pytest's JUnit report.

## Capabilities

### New Capabilities
- `training-data`: How the dataset is split into train, validation, and holdout sets, and the guarantees on those splits.
- `model-evaluation`: The metrics computed for a trained model and the rule that the holdout set is used only for final evaluation.
- `model-publishing`: How a trained model is stored, described, registered, and validated.

### Modified Capabilities

None.

## Impact

- **Python packages:** new `spotter-model` and `spotter-trainer` workspace members; PyTorch and torchvision (CPU wheels), boto3, grpcio.
- **Data:** EuroSAT RGB (27,000 public Sentinel-2 tiles, about 94 MB) downloaded to `data/`, which is gitignored.
- **Object store:** a `models` bucket.
- **CI:** Python tests write a JUnit report; spec coverage moves to its own job that reads the Go and Python reports together.
- **Out of scope:** GPU training, hyperparameter search, ONNX export and quantization (a later change), and retraining on degraded data (a later change).
