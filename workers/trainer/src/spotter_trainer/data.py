"""EuroSAT on disk: download (checksum-verified by torchvision), list, and load into memory."""

from pathlib import Path

import numpy as np
import torch
from PIL import Image
from spotter_model import CLASSES
from torchvision.datasets import EuroSAT


def download(root: Path) -> Path:
    """Download and unpack EuroSAT RGB under root if needed; return the images directory.

    torchvision verifies the archive's MD5 before unpacking.
    """
    root.mkdir(parents=True, exist_ok=True)
    EuroSAT(root=str(root), download=True)
    images = root / "eurosat" / "2750"
    if not images.is_dir():
        raise FileNotFoundError(f"expected EuroSAT images at {images}")
    return images


def list_images(images_dir: Path) -> dict[str, list[str]]:
    """Map each class name to its images' paths relative to images_dir (POSIX style)."""
    found = {}
    for name in CLASSES:
        class_dir = images_dir / name
        if not class_dir.is_dir():
            raise FileNotFoundError(f"missing class directory {class_dir}")
        found[name] = sorted(p.relative_to(images_dir).as_posix() for p in class_dir.glob("*.jpg"))
    return found


def load_images(images_dir: Path, paths: list[str]) -> torch.Tensor:
    """Decode images into one uint8 tensor of shape [N, 3, 64, 64] (design D2)."""
    out = torch.empty((len(paths), 3, 64, 64), dtype=torch.uint8)
    for i, rel in enumerate(paths):
        with Image.open(images_dir / rel) as img:
            arr = np.array(img.convert("RGB").resize((64, 64)), dtype=np.uint8)
        out[i] = torch.from_numpy(arr).permute(2, 0, 1)
    return out
