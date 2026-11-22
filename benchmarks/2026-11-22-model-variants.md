# Model variants benchmark, 2026-11-22

Source: `eurosat-classifier` version 1. Holdout: 4050 tiles (split `0f4c63a0ecca`).
Conditions: Intel64 Family 6 Model 170 Stepping 4, GenuineIntel, 22 logical cores, 4 threads, Python 3.13.14, PyTorch 2.15.0+cpu, ONNX Runtime 1.32.0.

| Variant | Size (KB) | Holdout accuracy | Macro F1 |
|---|---|---|---|
| pytorch-fp32 | 1143 | 0.9657 | 0.9644 |
| onnx-fp32 | 1157 | 0.9657 | 0.9644 |
| onnx-int8 | 320 | 0.9654 | 0.9640 |

| Variant | Batch | p50 ms | p95 ms | p99 ms | Images/s |
|---|---|---|---|---|---|
| pytorch-fp32 | 1 | 1.98 | 2.83 | 3.19 | 476 |
| pytorch-fp32 | 8 | 13.88 | 27.50 | 31.74 | 518 |
| pytorch-fp32 | 32 | 48.50 | 61.62 | 65.18 | 645 |
| onnx-fp32 | 1 | 0.81 | 1.02 | 1.18 | 1190 |
| onnx-fp32 | 8 | 6.57 | 12.35 | 13.41 | 1064 |
| onnx-fp32 | 32 | 24.23 | 28.82 | 29.76 | 1282 |
| onnx-int8 | 1 | 0.43 | 0.60 | 0.69 | 2239 |
| onnx-int8 | 8 | 2.68 | 3.30 | 3.77 | 2904 |
| onnx-int8 | 32 | 10.21 | 13.89 | 16.13 | 3011 |

ONNX fp32 parity with PyTorch: max probability difference 1.25e-06, 0 disagreeing predictions.
