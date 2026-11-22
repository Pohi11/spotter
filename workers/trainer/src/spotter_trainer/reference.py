"""Reference distributions for drift monitoring (decision record 0012).

For every validation image: the RGB channel means and standard deviations on a 0-1 scale,
computed exactly as the inference worker computes its input statistics (float32, population
standard deviation), plus the version's top-class confidence.
"""

import json

import numpy as np
import torch

from spotter_trainer.optimize import Predict, normalized, predict_all

FEATURES = ("mean_r", "mean_g", "mean_b", "contrast", "confidence")


def image_features(images_uint8: torch.Tensor) -> dict[str, np.ndarray]:
    """Per-image input statistics, matching the worker's `classify`."""
    pixels = images_uint8.numpy().astype(np.float32) / np.float32(255.0)  # [N, 3, H, W]
    means = pixels.mean(axis=(2, 3))
    stds = pixels.std(axis=(2, 3))
    return {
        "mean_r": means[:, 0],
        "mean_g": means[:, 1],
        "mean_b": means[:, 2],
        "contrast": stds.mean(axis=1),
    }


def build_reference(predict: Predict, images_uint8: torch.Tensor, version_id: str) -> dict:
    feats = image_features(images_uint8)
    feats["confidence"] = predict_all(predict, normalized(images_uint8)).max(axis=1)
    return {
        "version_id": version_id,
        "split": "validation",
        "count": int(len(images_uint8)),
        "features": {name: [round(float(v), 6) for v in feats[name]] for name in FEATURES},
    }


def reference_uri(artifact_uri: str) -> str:
    return artifact_uri.rsplit("/", 1)[0] + "/reference.json"


def write_reference(s3, artifact_uri: str, reference: dict) -> str:
    uri = reference_uri(artifact_uri)
    bucket, key = uri.removeprefix("s3://").split("/", 1)
    s3.put_object(
        Bucket=bucket, Key=key, Body=json.dumps(reference).encode(), ContentType="application/json"
    )
    return uri
