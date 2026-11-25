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


SCENARIO = ("dashboard", "Scenario tiles are a fixed sample of the holdout split")


@pytest.mark.spec(*SCENARIO)
def test_scenario_bundles_list_the_same_holdout_tiles(tmp_path):
    import base64

    from spotter_trainer import data
    from spotter_trainer.demo_samples import export_scenario
    from spotter_trainer.splits import split

    images = fake_eurosat(tmp_path / "2750", per_class=40)
    holdout = {
        p.split("/")[1].removesuffix(".jpg").lower(): p
        for p in split(data.list_images(images), seed=42).holdout
    }
    out = tmp_path / "scenario"
    bundles = export_scenario(images, out, count=30)

    assert set(bundles) == {"clean", *KINDS}
    ids = [t["id"] for t in bundles["clean"]["tiles"]]
    assert len(ids) == 30 and len(set(ids)) == 30 and set(ids) <= set(holdout)
    for variant, bundle in bundles.items():
        assert json.loads((out / f"{variant}.json").read_text(encoding="utf-8")) == bundle
        assert [t["id"] for t in bundle["tiles"]] == ids
        for i, tile in enumerate(bundle["tiles"]):
            source = (images / holdout[tile["id"]]).read_bytes()
            assert tile["cls"] == holdout[tile["id"]].split("/")[0]
            want = source if variant == "clean" else degrade_jpeg(source, variant, 1.0, seed=i)
            assert base64.b64decode(tile["jpeg"]) == want

    again = tmp_path / "again"
    export_scenario(images, again, count=30)
    for variant in bundles:
        assert (again / f"{variant}.json").read_bytes() == (out / f"{variant}.json").read_bytes()
