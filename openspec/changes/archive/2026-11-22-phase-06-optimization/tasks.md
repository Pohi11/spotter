# Tasks

## 1. Decision and pure logic

- [x] 1.1 Write decision record 0011 (ONNX Runtime, static int8).
- [x] 1.2 Add percentiles and the tolerance gate as pure functions. Verify their scenarios.

## 2. Optimization tool

- [x] 2.1 Add export with the parity check, static quantization, accuracy evaluation, and benchmarking. Verify the export scenarios on a small model.
- [x] 2.2 Add the report writer and `spotter-optimize` (publish and register variants with lineage).

## 3. Worker

- [x] 3.1 Serve `onnx` versions with ONNX Runtime. Verify the ONNX scenario against Compose infrastructure.

## 4. Load generator

- [x] 4.1 Add `cmd/spotterload`. Verify its scenarios with fakes.

## 5. Real measurements

- [x] 5.1 Run `spotter-optimize` on version 1; commit the benchmark report. Run `spotterload` with the PyTorch and the int8 version in production; record both.
