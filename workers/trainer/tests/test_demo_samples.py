import io
import json

import numpy as np
import pytest
from PIL import Image
from spotter_model import CLASSES
from spotter_trainer.degrade import KINDS, degrade_jpeg
from spotter_trainer.demo_samples import STRENGTHS, export

REQ = ("dashboard", "Demo samples are generated with the drift experiment's degradations")


def fake_eurosat(root, per_class=20):
    rng = np.random.default_rng(3)
    for name in CLASSES:
        d = root / name
        d.mkdir(parents=True)
        for i in range(per_class):
            buf = io.BytesIO()
            arr = rng.integers(0, 255, (64, 64, 3), dtype=np.uint8)
            Image.fromarray(arr).save(buf, format="JPEG", quality=95)
            (d / f"{name}_{i + 1}.jpg").write_bytes(buf.getvalue())
    return root


@pytest.mark.spec(*REQ)
def test_manifest_matches_files_and_degradations(tmp_path):
    images = fake_eurosat(tmp_path / "2750")
    out = tmp_path / "samples"
    manifest = export(images, out)

    assert json.loads((out / "manifest.json").read_text(encoding="utf-8")) == manifest
    assert [s["true_class"] for s in manifest["samples"]] == list(CLASSES)
    for sample in manifest["samples"]:
        source = (images / sample["source"]).read_bytes()
        kinds = {(v["degradation"], v["strength"]) for v in sample["variants"]}
        assert kinds == {("none", 0.0)} | {(k, s) for k in KINDS for s in STRENGTHS}
        for v in sample["variants"]:
            written = (out / v["file"]).read_bytes()
            if v["degradation"] == "none":
                assert written == source
            else:
                assert written == degrade_jpeg(source, v["degradation"], v["strength"], seed=0)


@pytest.mark.spec(*REQ)
def test_samples_come_from_the_holdout_split(tmp_path):
    from spotter_trainer import data
    from spotter_trainer.splits import split

    images = fake_eurosat(tmp_path / "2750")
    holdout = set(split(data.list_images(images), seed=42).holdout)
    manifest = export(images, tmp_path / "samples")
    assert all(s["source"] in holdout for s in manifest["samples"])
