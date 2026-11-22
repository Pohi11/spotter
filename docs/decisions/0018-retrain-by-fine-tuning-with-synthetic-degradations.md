# 0018 — Retrain by fine-tuning on synthetic degradations, triggered by drift, up to shadow

- **Date:** 2026-11-22
- **Status:** Accepted

## Context

The monitor raises drift from unlabeled traffic. Production traffic has no labels, and the project uses only public and synthetic data, so a new version can learn only from the labeled EuroSAT training split. The drift experiment showed which shifts the monitor sees (darkening, haze, noise, blur) and that the production model misclassifies such tiles (a fully hazed forest tile comes back as sea or lake). The platform also has to decide who may start a retraining and how far its result may go.

## Options considered

1. **Retrain from scratch on the clean split.** Reproduces the same model; nothing learned about the shift. About 18 minutes on a laptop CPU.
2. **Fine-tune the newest PyTorch version with the drift experiment's degradations as augmentation.** Minutes on a CPU, and the training distribution covers the shifts the monitor flags.
3. **Pseudo-label drifted production tiles with the current model and train on them.** Teaches the model its own mistakes; a hazed forest labeled "sea or lake" would be learned as such.

For the trigger: a person starts every run; or a consumer of `DriftDetected` starts one per event.

For the outcome: register only; register and validate; or go as far as shadow.

## Decision

Option 2, triggered per `DriftDetected` event by an optional watcher (or run by hand or as a Kubernetes Job), going as far as `shadow`. Shadow traffic, the code gates, and a person's approval still decide promotion.

## Consequences

- **Good:** The loop closes: drift detected, a candidate trained and shadowed without anyone starting it, and a person approving with the gate report and per-degradation accuracy in front of them.
- **Good:** Retraining can't promote anything; the shadow stage is safe by design because only production's answers are returned.
- **Cost:** The augmentation guesses the shift. A shift unlike the four degradations isn't learned, and the metrics say so only for the four.
- **Cost:** A drift that persists after retraining raises no new event until it resolves, so the watcher retrains once per drift episode, not continuously.
