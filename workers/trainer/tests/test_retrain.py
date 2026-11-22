import json

import numpy as np
import pytest
import torch
from spotter.v1 import registry_events_pb2 as events
from spotter_trainer.degrade import KINDS
from spotter_trainer.retrain import degrade_set, make_augment, register_retrained, robustness

from .test_publish import bucket, get, s3  # noqa: F401 - fixtures
from .test_registry import FakeRegistry, registry  # noqa: F401 - fixtures

AUGMENT = ("model-retraining", "Retraining fine-tunes with synthetic degradations")
EVALUATED = (
    "model-retraining",
    "A retrained version is evaluated on clean and degraded holdout tiles",
)
SHADOW = ("model-retraining", "A retrained version reaches shadow at most")


def tiles(n=32, seed=0) -> torch.Tensor:
    g = torch.Generator().manual_seed(seed)
    return torch.randint(40, 215, (n, 3, 64, 64), generator=g, dtype=torch.int64).to(torch.uint8)


def flips(t: torch.Tensor) -> list[torch.Tensor]:
    return [t, t.flip(-1), t.flip(-2), t.flip(-1).flip(-2)]


@pytest.mark.spec(*AUGMENT)
def test_probability_zero_only_flips():
    x = tiles()
    out = make_augment(0.0)(x, torch.Generator().manual_seed(1))
    for orig, aug in zip(x, out, strict=True):
        assert any(torch.equal(aug, f) for f in flips(orig))


@pytest.mark.spec(*AUGMENT)
def test_probability_one_degrades_every_tile():
    x = tiles()
    out = make_augment(1.0)(x, torch.Generator().manual_seed(1))
    for orig, aug in zip(x, out, strict=True):
        assert not any(torch.equal(aug, f) for f in flips(orig))


@pytest.mark.spec(*AUGMENT)
def test_same_seed_same_tiles():
    x = tiles()
    a = make_augment(0.5)(x, torch.Generator().manual_seed(7))
    b = make_augment(0.5)(x, torch.Generator().manual_seed(7))
    assert torch.equal(a, b)
    assert torch.equal(x, tiles())  # the input is not modified


def bright_predictor(images: torch.Tensor) -> np.ndarray:
    """Class 1 for bright tiles, 0 for dark ones: fooled by darkening, as a model can be."""
    return (images.float().mean(dim=(1, 2, 3)) > 100).numpy().astype(np.int64)


@pytest.mark.spec(*EVALUATED)
def test_robustness_covers_clean_and_each_degradation():
    x = tiles()
    labels = bright_predictor(x)
    r = robustness(bright_predictor, x, labels)
    assert set(r) == {"accuracy"} | {f"accuracy_{k}" for k in KINDS}
    assert r["accuracy"] == 1.0
    assert r["accuracy_darken"] < 1.0  # half brightness flips the bright tiles
    assert torch.equal(degrade_set(x, "darken"), degrade_set(x, "darken"))


METRICS = {
    "accuracy": 0.95,
    "accuracy_blur": 0.8,
    "accuracy_noise": 0.7,
    "accuracy_haze": 0.9,
    "accuracy_darken": 0.92,
}


@pytest.mark.integration
@pytest.mark.spec(*SHADOW)
@pytest.mark.spec(*EVALUATED)
def test_accurate_version_goes_to_shadow(s3, bucket, registry):  # noqa: F811
    fake, client = registry
    client.ensure_model("eurosat-classifier", "test")
    meta = {
        "parent_version": 1,
        "parent_metrics": METRICS | {"accuracy_haze": 0.2},
        "trigger": {"description": "drift"},
    }
    out = register_retrained(
        s3,
        bucket,
        client._stub,
        "eurosat-classifier",
        b"weights",
        METRICS,
        meta,
        0.85,
        "drift detected",
    )

    assert out.stage == "shadow"
    assert [t.to_stage for t in fake.transitions] == [
        events.MODEL_VERSION_STAGE_VALIDATED,
        events.MODEL_VERSION_STAGE_SHADOW,
    ]
    v = fake.versions["eurosat-classifier"][0]
    assert v.stage == events.MODEL_VERSION_STAGE_SHADOW and v.framework == "pytorch"
    assert dict(v.metrics) == pytest.approx(METRICS)
    stored = json.loads(get(s3, v.artifact_uri.rsplit("/", 1)[0] + "/metadata.json"))
    assert stored["parent_version"] == 1 and stored["parent_metrics"]["accuracy_haze"] == 0.2
    assert stored["metrics"] == METRICS


@pytest.mark.integration
@pytest.mark.spec(*SHADOW)
def test_below_floor_stays_registered(s3, bucket, registry):  # noqa: F811
    fake, client = registry
    client.ensure_model("eurosat-classifier", "test")
    out = register_retrained(
        s3,
        bucket,
        client._stub,
        "eurosat-classifier",
        b"weights",
        METRICS | {"accuracy": 0.80},
        {},
        0.85,
        "drift",
    )
    assert out.stage == "registered"
    assert fake.transitions == []
