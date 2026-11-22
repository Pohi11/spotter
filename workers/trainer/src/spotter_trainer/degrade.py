"""Deterministic, graded image degradations for drift experiments (design D6 of the drift
monitor change). Strength 0 leaves an image unchanged; strength 1 is the maximum."""

import io

import numpy as np
from PIL import Image, ImageFilter

KINDS = ("darken", "blur", "noise", "haze")
HAZE = np.array([200, 200, 200], dtype=np.float32)


def degrade(image: Image.Image, kind: str, strength: float, seed: int = 0) -> Image.Image:
    if not 0 <= strength <= 1:
        raise ValueError("strength must be between 0 and 1")
    rgb = image.convert("RGB")
    if kind == "blur":
        return rgb.filter(ImageFilter.GaussianBlur(radius=2 * strength)) if strength else rgb
    arr = np.asarray(rgb, dtype=np.float32)
    if kind == "darken":
        arr = arr * (1 - 0.5 * strength)
    elif kind == "noise":
        rng = np.random.default_rng(seed)
        arr = arr + rng.normal(0, 0.15 * 255 * strength, arr.shape)
    elif kind == "haze":
        arr = arr * (1 - 0.6 * strength) + HAZE * 0.6 * strength
    else:
        raise ValueError(f"unknown degradation {kind!r}; expected one of {KINDS}")
    return Image.fromarray(np.clip(arr, 0, 255).round().astype(np.uint8))


def degrade_jpeg(data: bytes, kind: str, strength: float, seed: int = 0) -> bytes:
    with Image.open(io.BytesIO(data)) as img:
        out = degrade(img, kind, strength, seed)
    buf = io.BytesIO()
    out.save(buf, format="JPEG", quality=95)
    return buf.getvalue()
