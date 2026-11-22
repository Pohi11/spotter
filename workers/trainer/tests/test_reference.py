import io
import json

import numpy as np
import pytest
import torch
from PIL import Image
from spotter_inference.models import LoadedModel
from spotter_inference.processor import classify
from spotter_model import EuroSATNet
from spotter_trainer.data import load_images
from spotter_trainer.optimize import torch_predictor
from spotter_trainer.reference import FEATURES, build_reference, image_features, write_reference

from .test_publish import bucket, get, s3  # noqa: F401 - fixtures

REFERENCE = ("model-publishing", "Each version carries a reference distribution")


def jpegs(tmp_path, n=6):
    rng = np.random.default_rng(0)
    paths = []
    for i in range(n):
        arr = rng.integers(0, 256, (64, 64, 3), dtype=np.uint8)
        Image.fromarray(arr).save(tmp_path / f"t{i}.jpg", quality=90)
        paths.append(f"t{i}.jpg")
    return paths


@pytest.mark.spec(*REFERENCE)
def test_same_features_as_serving(tmp_path):
    """The reference must describe images exactly as the worker does at serving time."""
    paths = jpegs(tmp_path)
    feats = image_features(load_images(tmp_path, paths))
    net = EuroSATNet().eval()
    model = LoadedModel("v", 1, "pytorch", torch_predictor(net))
    for i, rel in enumerate(paths):
        with Image.open(io.BytesIO((tmp_path / rel).read_bytes())) as img:
            img.load()
            _, stats, _ = classify(model, img.copy())
        assert abs(feats["mean_r"][i] - stats.channel_mean[0]) < 1e-6
        assert abs(feats["mean_g"][i] - stats.channel_mean[1]) < 1e-6
        assert abs(feats["mean_b"][i] - stats.channel_mean[2]) < 1e-6
        assert abs(feats["contrast"][i] - float(np.mean(stats.channel_std))) < 1e-6


@pytest.mark.integration
@pytest.mark.spec(*REFERENCE)
def test_reference_written_with_a_version(s3, bucket, tmp_path):  # noqa: F811
    images = load_images(tmp_path, jpegs(tmp_path, n=8))
    ref = build_reference(torch_predictor(EuroSATNet().eval()), images, "version-id")
    uri = write_reference(s3, f"s3://{bucket}/m/abc/model.pt", ref)
    assert uri == f"s3://{bucket}/m/abc/reference.json"
    stored = json.loads(get(s3, uri))
    assert stored["count"] == 8 and set(stored["features"]) == set(FEATURES)
    assert all(len(stored["features"][f]) == 8 for f in FEATURES)
    assert all(0 < c <= 1 for c in stored["features"]["confidence"])


def test_features_shape():
    feats = image_features(torch.zeros(3, 3, 64, 64, dtype=torch.uint8))
    assert set(feats) == {"mean_r", "mean_g", "mean_b", "contrast"}
    assert all(v.shape == (3,) and (v == 0).all() for v in feats.values())
