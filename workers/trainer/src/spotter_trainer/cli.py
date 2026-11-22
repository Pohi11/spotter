"""`spotter-train`: download, split, train, evaluate, publish, register, validate.

Configuration from environment variables:
  DATA_DIR (data), MODEL_NAME (eurosat-classifier), EPOCHS (12), SEED (42),
  MIN_ACCURACY (0.85), REGISTRY_ADDR (localhost:9090),
  S3_ENDPOINT, S3_ACCESS_KEY, S3_SECRET_KEY, S3_REGION (us-east-1), S3_BUCKET (models).
"""

import json
import os
import sys
from pathlib import Path

import boto3
import grpc
import torch
from spotter_model import CLASSES, EuroSATNet

from spotter_trainer import data
from spotter_trainer.publish import artifact_bytes, build_metadata, publish
from spotter_trainer.registry import RegistryClient
from spotter_trainer.splits import split
from spotter_trainer.train import Hyperparams, ImageSet, evaluate_holdout, train


def _env(key: str, default: str | None = None) -> str:
    value = os.environ.get(key, default)
    if value is None:
        sys.exit(f"spotter-train: {key} is required")
    return value


def _image_set(images_dir: Path, paths: tuple[str, ...]) -> ImageSet:
    labels = torch.tensor([CLASSES.index(p.split("/", 1)[0]) for p in paths], dtype=torch.int64)
    return ImageSet(data.load_images(images_dir, list(paths)), labels)


def main() -> None:
    hp = Hyperparams(epochs=int(_env("EPOCHS", "12")), seed=int(_env("SEED", "42")))
    model_name = _env("MODEL_NAME", "eurosat-classifier")
    floor = float(_env("MIN_ACCURACY", "0.85"))

    images_dir = data.download(Path(_env("DATA_DIR", "data")))
    s = split(data.list_images(images_dir), seed=hp.seed)
    print(
        f"split {s.fingerprint()[:12]}: {len(s.train)} train, {len(s.val)} val, "
        f"{len(s.holdout)} holdout",
        flush=True,
    )

    train_set, val_set = _image_set(images_dir, s.train), _image_set(images_dir, s.val)
    model = EuroSATNet()
    result = train(
        model,
        train_set,
        val_set,
        hp,
        on_epoch_end=lambda r: print(
            json.dumps({k: round(v, 4) for k, v in r.items()}), flush=True
        ),
    )
    # The holdout set is decoded only now, after training (design D4).
    report = evaluate_holdout(model, _image_set(images_dir, s.holdout), len(CLASSES))

    metrics = {
        "accuracy": report.accuracy,
        "macro_f1": report.macro_f1,
        "val_accuracy": result.best_val_accuracy,
    }
    metrics |= {f"recall_{c}": float(r) for c, r in zip(CLASSES, report.recall, strict=True)}
    metrics = {k: round(v, 4) for k, v in metrics.items()}
    print(
        json.dumps(
            {
                "holdout": metrics,
                "best_epoch": result.best_epoch,
                "confusion": report.confusion.tolist(),
            }
        ),
        flush=True,
    )

    s3 = boto3.client(
        "s3",
        endpoint_url=_env("S3_ENDPOINT"),
        aws_access_key_id=_env("S3_ACCESS_KEY"),
        aws_secret_access_key=_env("S3_SECRET_KEY"),
        region_name=_env("S3_REGION", "us-east-1"),
    )
    published = publish(
        s3,
        _env("S3_BUCKET", "models"),
        model_name,
        artifact_bytes(model),
        build_metadata(
            split_fingerprint=s.fingerprint(),
            seed=hp.seed,
            hyperparams=hp.as_dict() | {"best_epoch": result.best_epoch},
            classes=CLASSES,
            training_seconds=result.seconds,
            metrics=metrics,
        ),
    )
    print(f"published {published.uri}", flush=True)

    with grpc.insecure_channel(_env("REGISTRY_ADDR", "localhost:9090")) as channel:
        registry = RegistryClient(channel)
        registry.ensure_model(model_name, "EuroSAT land-use classifier (10 classes, 64x64 RGB)")
        version = registry.register_version(
            model_name, published.uri, published.sha256, "pytorch", metrics
        )
        validated = registry.validate_if_accurate(version, report.accuracy, floor)
    stage = "validated" if validated else "registered (below the accuracy floor)"
    print(f"registered {model_name} version {version.version}: {stage}", flush=True)


if __name__ == "__main__":
    main()
