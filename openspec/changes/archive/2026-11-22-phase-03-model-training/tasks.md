# Tasks

## 1. Traceability for Python

- [x] 1.1 Add the root `conftest.py` spec-marker hook and `speccheck -junit`. Verify with speccheck's tests and a JUnit report containing a marked test.

## 2. Shared model package

- [x] 2.1 Add `spotter_model`: `CLASSES`, `EuroSATNet`, `preprocess`. Verify with its tests (output shape, preprocessing of a 64×64 and a non-64×64 image).

## 3. Trainer

- [x] 3.1 Add `splits.py`. Verify every `training-data` scenario.
- [x] 3.2 Add `metrics.py`. Verify every `model-evaluation` metrics scenario.
- [x] 3.3 Add `data.py` and `train.py`. Verify the holdout-untouched scenario with a small synthetic dataset.
- [x] 3.4 Add `publish.py` and `registry.py`. Verify against a local SeaweedFS and an in-process fake registry server.
- [x] 3.5 Add `cli.py` (`spotter-train`).

## 4. Real run

- [x] 4.1 Download EuroSAT, train, publish to the local SeaweedFS, and register with the local registry. Record holdout metrics in the interview notes.

## 5. CI

- [x] 5.1 Python job writes JUnit; a spec-coverage job reads both reports. Verify every requirement is covered.
