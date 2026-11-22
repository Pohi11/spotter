"""Content-addressed artifact publishing with a metadata file (decision record 0008)."""

import hashlib
import io
import json
import platform
import subprocess
from dataclasses import dataclass

import torch
import torchvision
from botocore.exceptions import ClientError
from torch import nn


@dataclass(frozen=True)
class Published:
    uri: str
    sha256: str
    metadata_uri: str


def artifact_bytes(model: nn.Module) -> bytes:
    """The model's weights as torch.save bytes."""
    buf = io.BytesIO()
    torch.save(model.state_dict(), buf)
    return buf.getvalue()


def code_commit() -> str:
    """The current git commit, with "-dirty" if there are uncommitted changes."""
    try:
        commit = subprocess.run(
            ["git", "rev-parse", "HEAD"], capture_output=True, text=True, check=True
        ).stdout.strip()
        dirty = subprocess.run(
            ["git", "status", "--porcelain"], capture_output=True, text=True, check=True
        ).stdout.strip()
        return commit + ("-dirty" if dirty else "")
    except (OSError, subprocess.CalledProcessError):
        return "unknown"


def build_metadata(
    *,
    split_fingerprint: str,
    seed: int,
    hyperparams: dict,
    classes: tuple[str, ...],
    training_seconds: float,
    metrics: dict[str, float],
    commit: str | None = None,
) -> dict:
    return {
        "split_fingerprint": split_fingerprint,
        "seed": seed,
        "hyperparams": hyperparams,
        "classes": list(classes),
        "code_commit": commit or code_commit(),
        "python_version": platform.python_version(),
        "torch_version": torch.__version__,
        "torchvision_version": torchvision.__version__,
        "training_seconds": round(training_seconds, 1),
        "metrics": metrics,
    }


def ensure_bucket(s3, bucket: str) -> None:
    try:
        s3.head_bucket(Bucket=bucket)
    except ClientError as e:
        if e.response["Error"]["Code"] not in ("404", "NoSuchBucket", "NotFound"):
            raise
        s3.create_bucket(Bucket=bucket)


def publish(s3, bucket: str, model_name: str, data: bytes, metadata: dict) -> Published:
    """Upload data to <bucket>/<model_name>/<sha256>/model.pt with metadata.json beside it."""
    sha = hashlib.sha256(data).hexdigest()
    prefix = f"{model_name}/{sha}"
    ensure_bucket(s3, bucket)
    s3.put_object(Bucket=bucket, Key=f"{prefix}/model.pt", Body=data)
    body = json.dumps({**metadata, "artifact_sha256": sha}, indent=2, sort_keys=True).encode()
    s3.put_object(
        Bucket=bucket, Key=f"{prefix}/metadata.json", Body=body, ContentType="application/json"
    )
    return Published(
        uri=f"s3://{bucket}/{prefix}/model.pt",
        sha256=sha,
        metadata_uri=f"s3://{bucket}/{prefix}/metadata.json",
    )
