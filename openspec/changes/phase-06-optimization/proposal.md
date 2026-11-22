# Proposal

## Why

The model is accurate, but nobody has measured how fast it is, what it would cost to make it faster, or how the whole system behaves under load. National-security deployments often run on constrained or edge hardware, so the platform must be able to produce a smaller, faster model, prove what accuracy it gives up, and serve it — and every claim about speed must come from a measurement, not an estimate.

## What Changes

- Add `spotter-optimize`: export a registered PyTorch version to ONNX, check the export against PyTorch on every holdout image, quantize it to int8 (static, calibrated on training images), measure holdout accuracy and latency for each variant at several batch sizes, and write a dated benchmark report.
- Publish the ONNX variants as content-addressed artifacts and register them as new versions with their own holdout metrics. A quantized version is validated only if its accuracy loss stays within a configured tolerance.
- The worker serves `onnx` versions through ONNX Runtime as well as `pytorch` versions through PyTorch.
- Add `spotterload`, a Go load generator that drives the gateway at a set concurrency for a set duration and reports throughput and end-to-end latency percentiles, timing each job from submission to its completion event.

## Capabilities

### New Capabilities
- `model-optimization`: Exporting and quantizing a model version, the checks each variant must pass, the benchmark report, and registration of variants.
- `load-testing`: Driving the system with concurrent inference requests and reporting what was measured.

### Modified Capabilities
- `inference-processing`: the worker loads `onnx` artifacts with ONNX Runtime, in addition to `pytorch` artifacts.

## Impact

- **Python:** `onnx`, `onnxscript`, `onnxruntime` (trainer); `onnxruntime` (worker).
- **Go:** new `cmd/spotterload`.
- **Repository:** dated benchmark reports under `benchmarks/`.
- **Out of scope:** GPU runtimes (TensorRT), pruning and distillation, batching inside the worker (requests are still classified one at a time), autoscaling.
