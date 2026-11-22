# 0011 — Optimized serving with ONNX Runtime and static int8 quantization

- **Date:** 2026-11-22
- **Status:** Accepted

## Context

The platform should be able to make a model smaller and faster for constrained hardware, measure what that costs in accuracy, and serve the result — on CPUs, without a GPU, and without tying models to one framework. The first model is a small convolutional network trained in PyTorch (decision record [0008](0008-content-addressed-model-artifacts.md) covers how its artifacts are stored).

## Options considered

1. **Keep PyTorch eager.** No new dependency; slowest, and serving needs the full PyTorch install.
2. **TorchScript or `torch.compile`.** Faster than eager, still PyTorch-only at serving time.
3. **ONNX + ONNX Runtime.** A framework-neutral artifact (models from PyTorch, TensorFlow, and others export to it) and a small, optimized CPU runtime; supports int8 quantization.
4. **OpenVINO or TensorRT.** Excellent on their target hardware (Intel CPUs, NVIDIA GPUs) but hardware-specific.

For quantization:

5. **Dynamic quantization.** No calibration data, but quantizes only MatMul/Linear; this model is almost all convolutions.
6. **Static quantization (QDQ, per-channel).** Needs calibration data (a few hundred training images); quantizes convolutions and activations.

## Decision

Options 3 and 6.

- Optimized variants are ONNX artifacts with framework `onnx`, registered as new versions next to their PyTorch parent.
- int8 variants use static, per-channel QDQ quantization calibrated on training images only.
- Each variant must pass an export parity check against PyTorch and, if quantized, an accuracy tolerance against its parent before it is validated.
- The worker serves both PyTorch and ONNX versions.

## Consequences

- **Good:** A framework-neutral serving format; the worker could serve models trained elsewhere.
- **Good:** Every speedup comes with a measured accuracy cost, on the same holdout set.
- **Cost:** Two runtimes in the worker image.
- **Cost:** Calibration data must be representative; drift (a later change) may make an int8 model degrade faster than its parent.
