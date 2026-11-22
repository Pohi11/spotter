# Design

## Context

Version 1 (PyTorch, 96.57% holdout accuracy) is registered and served (changes `phase-03-model-training`, `phase-04-event-driven-inference`). The runtime choice is recorded in decision record [0011](../../../../docs/decisions/0011-onnx-runtime-static-int8.md).

## Decisions

### D1. One tool does export, check, quantize, measure, and register
`spotter-optimize` (in the trainer package, which already owns the data split and metrics) takes a source version, rebuilds the seed-42 split, and produces `onnx-fp32` and `onnx-int8` variants. It refuses to publish anything unless the export check passes.

### D2. Export: dynamic batch axis, fixed opset
`torch.onnx.export` with a dynamic batch dimension so one artifact serves batch sizes 1–N. The parity check runs both models on all 4,050 holdout tiles and compares softmax outputs.

### D3. Static int8 quantization in QDQ format, per-channel weights
`onnxruntime.quantization.quantize_static` with 512 calibration tiles drawn from the training split (never validation or holdout), per-channel weight scales, and QDQ nodes. Dynamic quantization was not used: it only quantizes MatMul/Linear, and this model is almost all convolutions.

### D4. Benchmarking conditions are fixed and recorded
Every variant runs with 4 intra-op threads (`torch.set_num_threads(4)`, `SessionOptions.intra_op_num_threads = 4`), 10 warm-up iterations, then timed iterations (200 at batch 1, 100 at batch 8, 50 at batch 32) on real holdout tiles. Percentiles use linear interpolation. The report records CPU model, logical cores, thread count, Python/PyTorch/ONNX Runtime versions, and date.

### D5. The tolerance gate is code
`validate_variant(parent_accuracy, variant_accuracy, tolerance, floor)` is a pure function with tests; the CLI applies it and transitions the version only when it passes. FP32 ONNX is held to the same rule (its accuracy should equal the parent's).

### D6. The worker hides the runtime behind one call
`LoadedModel.predict(batch: np.ndarray) -> np.ndarray` returns class probabilities; PyTorch and ONNX Runtime implementations differ only in how they're built from the artifact bytes. Classification code doesn't know which runtime it uses.

### D7. Load generator times jobs from completion events
`spotterload` submits over the gateway's gRPC API from N goroutines and subscribes (core NATS) to `inference.completed.<model>`; latency is the time from just before `SubmitInference` to the completion event's arrival. This excludes polling delay, which would otherwise dominate (`spotterctl` polls every 200 ms). The run loop depends on two small interfaces (submit, completions) so it is unit-tested with fakes.

## Risks and trade-offs

- **One worker classifies one image at a time,** so end-to-end throughput is bounded by the worker, not the model; the report shows both numbers. Worker-side batching is a later change.
- **Laptop CPU timings vary run to run.** Reports are dated and record their conditions; comparisons are made within one run.
