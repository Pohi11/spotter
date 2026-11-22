import numpy as np
import pytest
from PIL import Image
from spotter_trainer.degrade import KINDS, degrade


def tile(value=120) -> Image.Image:
    rng = np.random.default_rng(1)
    return Image.fromarray(rng.integers(value - 40, value + 40, (64, 64, 3), dtype=np.uint8))


@pytest.mark.parametrize("kind", KINDS)
def test_strength_zero_is_identity(kind):
    img = tile()
    assert np.array_equal(np.asarray(degrade(img, kind, 0.0)), np.asarray(img))


def test_darken_halves_brightness_at_full_strength():
    img = tile()
    before = np.asarray(img, dtype=np.float32).mean()
    after = np.asarray(degrade(img, "darken", 1.0), dtype=np.float32).mean()
    assert after == pytest.approx(before * 0.5, rel=0.01)


def test_blur_reduces_contrast_and_noise_increases_it():
    img = tile()
    std = np.asarray(img, dtype=np.float32).std()
    assert np.asarray(degrade(img, "blur", 1.0), dtype=np.float32).std() < std
    assert np.asarray(degrade(img, "noise", 1.0), dtype=np.float32).std() > std


def test_haze_moves_toward_light_grey():
    out = np.asarray(degrade(tile(60), "haze", 1.0), dtype=np.float32)
    assert out.mean() > 100


def test_noise_is_deterministic_for_a_seed():
    img = tile()
    assert np.array_equal(
        np.asarray(degrade(img, "noise", 0.5, seed=3)),
        np.asarray(degrade(img, "noise", 0.5, seed=3)),
    )
