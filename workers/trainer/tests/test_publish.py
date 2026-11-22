"""Publishing against a real S3-compatible store (SeaweedFS). Needs S3_ENDPOINT."""

import hashlib
import json
import os
import uuid

import boto3
import pytest
from spotter_model import CLASSES, EuroSATNet
from spotter_trainer.publish import artifact_bytes, build_metadata, publish

ADDRESSED = ("model-publishing", "Artifacts are content-addressed")
METADATA = ("model-publishing", "Every artifact carries reproducibility metadata")


@pytest.fixture
def s3():
    endpoint = os.environ.get("S3_ENDPOINT")
    if not endpoint:
        pytest.skip("S3_ENDPOINT not set")
    return boto3.client(
        "s3",
        endpoint_url=endpoint,
        aws_access_key_id=os.environ["S3_ACCESS_KEY"],
        aws_secret_access_key=os.environ["S3_SECRET_KEY"],
        region_name=os.environ.get("S3_REGION", "us-east-1"),
    )


@pytest.fixture
def bucket(s3):
    name = f"test-{uuid.uuid4().hex[:12]}"
    yield name
    for obj in s3.list_objects_v2(Bucket=name).get("Contents", []):
        s3.delete_object(Bucket=name, Key=obj["Key"])
    s3.delete_bucket(Bucket=name)


def metadata() -> dict:
    return build_metadata(
        split_fingerprint="ab" * 32,
        seed=42,
        hyperparams={"epochs": 1},
        classes=CLASSES,
        training_seconds=1.5,
        metrics={"accuracy": 0.9},
    )


def get(s3, uri: str) -> bytes:
    bucket, key = uri.removeprefix("s3://").split("/", 1)
    return s3.get_object(Bucket=bucket, Key=key)["Body"].read()


@pytest.mark.integration
@pytest.mark.spec(*ADDRESSED)
def test_location_and_checksum_agree(s3, bucket):
    data = artifact_bytes(EuroSATNet())
    p = publish(s3, bucket, "eurosat-classifier", data, metadata())
    stored = get(s3, p.uri)
    assert hashlib.sha256(stored).hexdigest() == p.sha256
    assert p.uri == f"s3://{bucket}/eurosat-classifier/{p.sha256}/model.pt"


@pytest.mark.integration
@pytest.mark.spec(*ADDRESSED)
def test_republishing_is_idempotent(s3, bucket):
    data = artifact_bytes(EuroSATNet())
    a = publish(s3, bucket, "eurosat-classifier", data, metadata())
    b = publish(s3, bucket, "eurosat-classifier", data, metadata())
    assert a.uri == b.uri
    keys = [o["Key"] for o in s3.list_objects_v2(Bucket=bucket)["Contents"]]
    assert sorted(keys) == sorted(
        [f"eurosat-classifier/{a.sha256}/model.pt", f"eurosat-classifier/{a.sha256}/metadata.json"]
    )


@pytest.mark.integration
@pytest.mark.spec(*METADATA)
def test_metadata_is_complete(s3, bucket):
    p = publish(s3, bucket, "eurosat-classifier", artifact_bytes(EuroSATNet()), metadata())
    meta = json.loads(get(s3, p.metadata_uri))
    for field in (
        "split_fingerprint",
        "seed",
        "hyperparams",
        "classes",
        "code_commit",
        "python_version",
        "torch_version",
        "torchvision_version",
        "training_seconds",
        "metrics",
        "artifact_sha256",
    ):
        assert meta.get(field) not in (None, "", [], {}), field
    assert meta["classes"] == list(CLASSES)
    assert meta["artifact_sha256"] == p.sha256
