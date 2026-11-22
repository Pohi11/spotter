# Spec Delta

## Purpose

Defines how a registered model version is turned into faster variants, what each variant must prove before it is trusted, how speed is measured and reported, and how variants are registered.

## ADDED Requirements

### Requirement: The ONNX export matches PyTorch
An ONNX export SHALL be checked against the PyTorch model on every holdout image. It SHALL be rejected unless the largest absolute difference between the two models' class probabilities is below 1e-4 and the predicted class is the same for every image.

#### Scenario: Faithful export
- **WHEN** a model is exported to ONNX
- **THEN** the check reports the largest probability difference and the number of disagreeing predictions, and passes only if they are below 1e-4 and zero

#### Scenario: Unfaithful export is rejected
- **WHEN** the exported model's outputs differ from PyTorch's by more than 1e-4
- **THEN** the export is rejected and nothing is published

### Requirement: Quantized versions are validated only within an accuracy tolerance
Each variant's holdout accuracy SHALL be measured on the same holdout set as its parent. A quantized variant SHALL be moved to `validated` only if its holdout accuracy is at least the parent's minus a configured tolerance (default 0.01) and at least the accuracy floor; otherwise it SHALL stay `registered`, and the reason SHALL state both accuracies.

#### Scenario: Within tolerance
- **WHEN** the parent's holdout accuracy is 0.9657 and the int8 variant's is 0.9640, with tolerance 0.01
- **THEN** the int8 version is moved to `validated`

#### Scenario: Too much accuracy lost
- **WHEN** the parent's holdout accuracy is 0.9657 and the int8 variant's is 0.9400, with tolerance 0.01
- **THEN** the int8 version stays `registered`

### Requirement: Benchmarks report percentiles, throughput, and their conditions
A benchmark SHALL report, for each variant and batch size, the p50, p95, and p99 latency per batch and the images per second, measured after warm-up iterations with a fixed number of threads, together with the holdout accuracy of the variant and the conditions of the run (CPU, thread count, library versions, date). The report SHALL be written as JSON and Markdown under `benchmarks/` with the date in its name.

#### Scenario: Report contents
- **WHEN** a benchmark runs over variants `pytorch-fp32`, `onnx-fp32`, `onnx-int8` and batch sizes 1, 8, 32
- **THEN** the report has a row for each of the nine combinations with p50, p95, p99, and images per second, and records the CPU, threads, and library versions

#### Scenario: Percentiles from samples
- **WHEN** latencies of 1, 2, 3, …, 100 ms are summarized
- **THEN** p50 is 50.5 ms, p95 is 95.05 ms, and p99 is 99.01 ms (linear interpolation)

### Requirement: Variants are registered as new versions with lineage
Each accepted variant SHALL be published as a content-addressed artifact with framework `onnx` and registered as a new version of the same model, with its own holdout metrics, and its `metadata.json` SHALL name the parent version and the optimization applied.

#### Scenario: Two variants registered
- **WHEN** optimization of version 1 succeeds
- **THEN** two new versions with framework `onnx` exist, and each one's metadata names version 1 as its parent and `fp32` or `int8-static` as its optimization
