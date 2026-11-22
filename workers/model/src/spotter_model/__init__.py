"""The EuroSAT classifier: class list, architecture, and preprocessing.

Shared by the trainer and the inference worker so they can never disagree on class order,
input size, or normalization.
"""

import numpy as np
import torch
from PIL import Image
from torch import nn

# Output order of the classifier. Matches torchvision's EuroSAT (sorted folder names).
CLASSES: tuple[str, ...] = (
    "AnnualCrop",
    "Forest",
    "HerbaceousVegetation",
    "Highway",
    "Industrial",
    "Pasture",
    "PermanentCrop",
    "Residential",
    "River",
    "SeaLake",
)

INPUT_SIZE = 64

# Per-channel mean and standard deviation of the training split (seed 42, 18,900 tiles,
# split fingerprint 0f4c63a0ecca…), on a 0-1 scale. Measured once and fixed here so
# serving needs no training data (design D5).
MEAN = (0.3446, 0.3806, 0.4078)
STD = (0.2025, 0.1370, 0.1157)


class EuroSATNet(nn.Module):
    """A small CNN for 64x64 RGB tiles: three conv blocks and a classifier head."""

    def __init__(self, num_classes: int = len(CLASSES)) -> None:
        super().__init__()

        def block(c_in: int, c_out: int) -> nn.Sequential:
            return nn.Sequential(
                nn.Conv2d(c_in, c_out, 3, padding=1, bias=False),
                nn.BatchNorm2d(c_out),
                nn.ReLU(inplace=True),
                nn.Conv2d(c_out, c_out, 3, padding=1, bias=False),
                nn.BatchNorm2d(c_out),
                nn.ReLU(inplace=True),
                nn.MaxPool2d(2),
            )

        self.features = nn.Sequential(block(3, 32), block(32, 64), block(64, 128))  # 64 -> 8
        self.head = nn.Sequential(
            nn.AdaptiveAvgPool2d(1),
            nn.Flatten(),
            nn.Dropout(0.3),
            nn.Linear(128, num_classes),
        )

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        return self.head(self.features(x))


_mean = torch.tensor(MEAN).view(1, 3, 1, 1)
_std = torch.tensor(STD).view(1, 3, 1, 1)


def normalize(batch_uint8: torch.Tensor) -> torch.Tensor:
    """Turn a uint8 batch [N, 3, H, W] into the float input the model expects."""
    return (batch_uint8.float() / 255.0 - _mean) / _std


def preprocess(image: Image.Image) -> torch.Tensor:
    """Turn one PIL image of any size or mode into a model input of shape [1, 3, 64, 64]."""
    rgb = image.convert("RGB")
    if rgb.size != (INPUT_SIZE, INPUT_SIZE):
        rgb = rgb.resize((INPUT_SIZE, INPUT_SIZE), Image.Resampling.BILINEAR)
    arr = np.array(rgb, dtype=np.uint8)  # a writable copy, as torch.from_numpy expects
    return normalize(torch.from_numpy(arr).permute(2, 0, 1).unsqueeze(0))
