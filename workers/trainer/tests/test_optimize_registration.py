"""Variants are published and registered with lineage (real SeaweedFS, in-process registry)."""

import json

import pytest
from spotter.v1 import registry_events_pb2 as events
from spotter.v1 import registry_pb2 as pb
from spotter_trainer.optimize_cli import register_variant

from .test_publish import bucket, get, s3  # noqa: F401 - fixtures
from .test_registry import FakeRegistry, registry  # noqa: F401 - fixtures

LINEAGE = ("model-optimization", "Variants are registered as new versions with lineage")


def variant(name: str, accuracy: float) -> dict:
    return {
        "name": name,
        "accuracy": accuracy,
        "macro_f1": accuracy - 0.001,
        "latency": [{"batch": 1, "p50_ms": 0.43, "p95_ms": 0.6, "p99_ms": 0.7}],
    }


@pytest.mark.integration
@pytest.mark.spec(*LINEAGE)
def test_two_variants_registered_with_lineage(s3, bucket, registry):  # noqa: F811
    fake, client = registry
    client.ensure_model("eurosat-classifier", "test")
    source = pb.ModelVersion(id="parent-id", version=1, framework="pytorch")
    parent_meta = {"split_fingerprint": "ab" * 32, "seed": 42, "classes": ["A", "B"]}
    stub = client._stub

    out = []
    for name, data, optimization, acc in (
        ("onnx-fp32", b"fp32-bytes", "fp32", 0.9657),
        ("onnx-int8", b"int8-bytes", "int8-static", 0.9654),
    ):
        out.append(
            register_variant(
                s3,
                bucket,
                stub,
                "eurosat-classifier",
                data,
                optimization,
                variant(name, acc),
                source,
                parent_meta,
                0.9657,
                0.01,
                0.85,
                "abc123",
            )
        )

    registered = fake.versions["eurosat-classifier"]
    assert len(registered) == 2 and all(v.framework == "onnx" for v in registered)
    for (version, ok, _), optimization in zip(out, ("fp32", "int8-static"), strict=True):
        assert ok
        meta = json.loads(get(s3, version.artifact_uri.rsplit("/", 1)[0] + "/metadata.json"))
        assert meta["parent_version"] == 1 and meta["parent_version_id"] == "parent-id"
        assert meta["optimization"] == optimization
        assert meta["split_fingerprint"] == parent_meta["split_fingerprint"]
    assert [t.to_stage for t in fake.transitions] == [events.MODEL_VERSION_STAGE_VALIDATED] * 2
