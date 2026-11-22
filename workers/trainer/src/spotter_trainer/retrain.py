"""Retraining a model version for drifted inputs (change retrain-on-drift, decision record
0018): fine-tune the newest PyTorch version with the drift experiment's degradations as
augmentation, measure it on clean and degraded holdout tiles, and take it as far as
shadow, never further."""

from collections.abc import Callable
from dataclasses import dataclass

import numpy as np
import torch
from PIL import Image
from spotter.v1 import registry_events_pb2 as events
from spotter.v1 import registry_pb2 as pb

from spotter_trainer.degrade import KINDS, degrade
from spotter_trainer.publish import publish
from spotter_trainer.train import _augment

MIN_STRENGTH = 0.25  # the weakest strength the drift experiment measured


def _degrade_tile(tile: torch.Tensor, kind: str, strength: float, seed: int) -> torch.Tensor:
    img = Image.fromarray(tile.permute(1, 2, 0).numpy())
    out = np.asarray(degrade(img, kind, strength, seed=seed), dtype=np.uint8)
    return torch.from_numpy(out.copy()).permute(2, 0, 1)


def make_augment(p: float) -> Callable[[torch.Tensor, torch.Generator], torch.Tensor]:
    """Flips, then with probability p one degradation per tile at a strength in [0.25, 1].

    Every random choice comes from the training generator, so a seed repeats a run.
    """
    if not 0 <= p <= 1:
        raise ValueError("degradation probability must be between 0 and 1")

    def augment(x: torch.Tensor, g: torch.Generator) -> torch.Tensor:
        x = _augment(x, g)
        n = x.shape[0]
        hit = torch.rand(n, generator=g) < p
        kinds = torch.randint(len(KINDS), (n,), generator=g)
        strengths = MIN_STRENGTH + (1 - MIN_STRENGTH) * (1 - torch.rand(n, generator=g))
        seeds = torch.randint(2**31 - 1, (n,), generator=g)
        for i in torch.nonzero(hit).flatten().tolist():
            x[i] = _degrade_tile(x[i], KINDS[kinds[i]], float(strengths[i]), int(seeds[i]))
        return x

    return augment


def degrade_set(images: torch.Tensor, kind: str, strength: float = 1.0) -> torch.Tensor:
    """The same tiles with one degradation applied, seeded by position (as the experiment)."""
    out = images.clone()
    for i in range(len(images)):
        out[i] = _degrade_tile(images[i], kind, strength, seed=i)
    return out


def robustness(
    predict: Callable[[torch.Tensor], np.ndarray], images: torch.Tensor, labels: np.ndarray
) -> dict[str, float]:
    """Accuracy on the clean tiles and on each degradation at full strength.

    predict maps uint8 tiles [N, 3, 64, 64] to predicted class indices.
    """
    out = {"accuracy": float((predict(images) == labels).mean())}
    for kind in KINDS:
        out[f"accuracy_{kind}"] = float((predict(degrade_set(images, kind)) == labels).mean())
    return {k: round(v, 4) for k, v in out.items()}


@dataclass(frozen=True)
class Outcome:
    version: pb.ModelVersion
    stage: str  # "shadow" or "registered"
    reason: str


def register_retrained(
    s3,
    bucket: str,
    stub,
    model_name: str,
    model_bytes: bytes,
    metrics: dict[str, float],
    metadata: dict,
    floor: float,
    trigger: str,
) -> Outcome:
    """Publish, register, validate if accurate enough, and move to shadow.

    The only transitions requested are to validated and shadow (decision record 0018).
    """
    published = publish(s3, bucket, model_name, model_bytes, metadata | {"metrics": metrics})
    version = stub.RegisterVersion(
        pb.RegisterVersionRequest(
            model_name=model_name,
            artifact_uri=published.uri,
            artifact_sha256=published.sha256,
            framework="pytorch",
            metrics=metrics,
        ),
        timeout=10,
    ).version
    if metrics["accuracy"] < floor:
        reason = f"holdout accuracy {metrics['accuracy']:.4f} is below the floor {floor:.2f}"
        return Outcome(version, "registered", reason)
    validated = stub.TransitionVersion(
        pb.TransitionVersionRequest(
            version_id=version.id,
            to_stage=events.MODEL_VERSION_STAGE_VALIDATED,
            reason=(
                f"retrained; holdout accuracy {metrics['accuracy']:.4f} meets the floor {floor:.2f}"
            ),
            expected_revision=version.revision,
        ),
        timeout=10,
    ).version
    shadowed = stub.TransitionVersion(
        pb.TransitionVersionRequest(
            version_id=version.id,
            to_stage=events.MODEL_VERSION_STAGE_SHADOW,
            reason=f"shadow after retraining: {trigger}",
            expected_revision=validated.revision,
        ),
        timeout=10,
    ).version
    return Outcome(shadowed, "shadow", trigger)
