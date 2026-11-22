# Drift detection, 2026-11-22

Window 200 events, evaluated every 50; drift at PSI ≥ 0.2, resolved below 0.1.
Clean traffic: **0 false alarms in 3000 events** (60 evaluations).

| Degradation | Strength | Detected | Degraded events to detect | Drifted features |
|---|---|---|---|---|
| darken | 0.25 | yes | 144 | mean_b, mean_g |
| darken | 0.5 | yes | 96 | mean_b, mean_g |
| darken | 1.0 | yes | 64 | mean_b, mean_g |
| blur | 0.25 | no | > 1000 |  |
| blur | 0.5 | yes | 167 | confidence |
| blur | 1.0 | yes | 215 | confidence, contrast |
| noise | 0.25 | yes | 161 | confidence, contrast |
| noise | 0.5 | yes | 137 | confidence |
| noise | 1.0 | yes | 62 | confidence |
| haze | 0.25 | yes | 150 | mean_b, mean_g, mean_r |
| haze | 0.5 | yes | 124 | mean_b, mean_g, mean_r |
| haze | 1.0 | yes | 130 | mean_b, mean_g, mean_r |

## What the numbers say

- Run against the live stack (gateway → JetStream → worker serving v3 int8 → monitor), 400 shuffled holdout tiles as clean traffic (natural class mix), degraded copies made with `spotter-degrade`.
- **No false alarms** in 60 evaluations of clean traffic.
- Counts are degraded events submitted before `DriftDetected` arrived; detection happens at 50-event evaluation boundaries, and up to 4 submissions were in flight, so each count is accurate to about ±50. Non-monotonic pairs (blur 0.5 vs 1.0, haze 0.5 vs 1.0) are within that granularity.
- **Darkening and haze** show up in the brightness features; **noise and blur** show up mostly in the model's confidence, the input statistics move little.
- **Mild blur (radius 0.5 px) was not detected** within 1,000 events: neither brightness, contrast, nor confidence moved enough. Catching it would need a sharpness feature.
- Separately, replaying earlier test traffic (a few hundred fixed tiles, balanced 20 per class) raised two borderline alerts (PSI 0.21–0.27 on `mean_r` and contrast): traffic whose class mix differs from the training data also registers as drift.
