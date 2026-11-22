"""`spotter-reference`: write reference.json for every version of a model that lacks one.

Configuration from environment variables: MODEL_NAME (eurosat-classifier), DATA_DIR (data),
SEED (42), REGISTRY_ADDR (localhost:9090), S3_ENDPOINT, S3_ACCESS_KEY, S3_SECRET_KEY,
S3_REGION (us-east-1).
"""

import hashlib
import io
import os
import sys
from pathlib import Path

import boto3
import grpc
import torch
from botocore.exceptions import ClientError
from spotter.v1 import registry_pb2 as pb
from spotter.v1 import registry_pb2_grpc as rpc
from spotter_model import EuroSATNet

from spotter_trainer import data
from spotter_trainer.optimize import onnx_predictor, torch_predictor
from spotter_trainer.reference import build_reference, reference_uri, write_reference
from spotter_trainer.splits import split


def _env(key: str, default: str | None = None) -> str:
    value = os.environ.get(key, default)
    if value is None:
        sys.exit(f"spotter-reference: {key} is required")
    return value


def predictor_for(framework: str, artifact: bytes):
    if framework == "onnx":
        return onnx_predictor(artifact)
    net = EuroSATNet()
    net.load_state_dict(torch.load(io.BytesIO(artifact), weights_only=True))
    return torch_predictor(net)


def main() -> None:
    model_name = _env("MODEL_NAME", "eurosat-classifier")
    s3 = boto3.client(
        "s3",
        endpoint_url=_env("S3_ENDPOINT"),
        aws_access_key_id=_env("S3_ACCESS_KEY"),
        aws_secret_access_key=_env("S3_SECRET_KEY"),
        region_name=_env("S3_REGION", "us-east-1"),
    )
    with grpc.insecure_channel(_env("REGISTRY_ADDR", "localhost:9090")) as channel:
        versions = (
            rpc.RegistryServiceStub(channel)
            .ListVersions(pb.ListVersionsRequest(model_name=model_name), timeout=10)
            .versions
        )

    images_dir = data.download(Path(_env("DATA_DIR", "data")))
    val = None
    for v in versions:
        bucket, key = reference_uri(v.artifact_uri).removeprefix("s3://").split("/", 1)
        try:
            s3.head_object(Bucket=bucket, Key=key)
            print(f"version {v.version}: reference exists")
            continue
        except ClientError:
            pass
        a_bucket, a_key = v.artifact_uri.removeprefix("s3://").split("/", 1)
        artifact = s3.get_object(Bucket=a_bucket, Key=a_key)["Body"].read()
        if hashlib.sha256(artifact).hexdigest() != v.artifact_sha256:
            sys.exit(f"spotter-reference: version {v.version} artifact checksum mismatch")
        if val is None:
            s = split(data.list_images(images_dir), seed=int(_env("SEED", "42")))
            val = data.load_images(images_dir, list(s.val))
        ref = build_reference(predictor_for(v.framework, artifact), val, v.id)
        print(f"version {v.version}: wrote {write_reference(s3, v.artifact_uri, ref)}")


if __name__ == "__main__":
    main()
