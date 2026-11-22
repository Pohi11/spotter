# Retraining on drift: real run (2026-11-22)

The full loop on the live stack (registry, gateway, monitor, and inference worker on the host; NATS, Postgres, and SeaweedFS in Docker), with `spotter-retrain --watch` running. Version 3 (ONNX int8) was in production. Raw numbers: [2026-11-22-retraining.json](2026-11-22-retraining.json).

## Timeline

| Time (UTC) | What happened | Since the hazed traffic began |
|---|---|---|
| 22:48:15 | `spotterload` starts sending fully hazed holdout tiles | 0 s |
| 22:48:18.4 | The monitor publishes `DriftDetected` for version 3 (mean_r PSI 0.22, mean_g 0.43, mean_b 0.39) | 3.4 s |
| 22:48:21.9 | The retrainer receives it and starts fine-tuning version 1 (the newest PyTorch version) | 6.9 s |
| 22:55:18.6 | Version 4 registered, validated, and moved to shadow | 7 min 3.6 s |
| 22:55:19.1 | The worker loads version 4 as its shadow | 7 min 4.1 s |

Training took 308.5 s (3 epochs on the 18,900 training tiles, half of them degraded); the rest is loading the dataset and evaluating both models on the holdout, clean and degraded.

## Accuracy on the 4,050 holdout tiles

| Holdout | Version 1 (parent) | Version 4 (retrained) |
|---|---|---|
| Clean | 96.57% | 95.58% |
| Haze, full strength | 22.32% | 87.80% |
| Darken, full strength | 31.28% | 88.86% |
| Blur, full strength | 24.54% | 75.19% |
| Noise, full strength | 11.14% | 64.47% |

The retrained version gives up about one point of clean accuracy for large gains on every degradation.

## Gates after clean traffic

After 15 s of clean traffic (4 clients), the registry reported for version 4: accuracy passed (0.9558 against production's 0.9654, tolerance 0.01), shadow agreement passed (592 comparisons, 98.5%), shadow latency passed (mean 7.6 ms, budget 25 ms), approval not yet recorded. Promotion waits for a person.
