"""`spotter-retrain`: fine-tune the newest PyTorch version for drifted inputs and take the
result as far as shadow (change retrain-on-drift, decision record 0018).

  spotter-retrain           one run now
  spotter-retrain --watch   one run per DriftDetected event on the model, until stopped

Configuration from environment variables:
  DATA_DIR (data), MODEL_NAME (eurosat-classifier), EPOCHS (3), LEARNING_RATE (5e-4),
  DEGRADE_P (0.5), MIN_ACCURACY (0.85), REGISTRY_ADDR (localhost:9090), NATS_URL (watch),
  S3_ENDPOINT, S3_ACCESS_KEY, S3_SECRET_KEY, S3_REGION (us-east-1), S3_BUCKET (models).
"""

import argparse
import asyncio
import copy
import io
import json
import logging
import os
import signal
import sys
from collections.abc import Callable
from pathlib import Path

import boto3
import grpc
import nats
import numpy as np
import torch
from nats.js.api import ConsumerConfig, DeliverPolicy
from spotter.v1 import monitor_pb2 as mpb
from spotter.v1 import registry_pb2 as pb
from spotter.v1 import registry_pb2_grpc as rpc
from spotter_model import CLASSES, EuroSATNet

from spotter_trainer import data
from spotter_trainer.optimize import torch_predictor
from spotter_trainer.publish import artifact_bytes, code_commit
from spotter_trainer.reference import build_reference, write_reference
from spotter_trainer.retrain import make_augment, register_retrained, robustness
from spotter_trainer.splits import split
from spotter_trainer.train import Hyperparams, ImageSet, predict, train

log = logging.getLogger("spotter_retrain")


def _env(key: str, default: str | None = None) -> str:
    value = os.environ.get(key, default)
    if value is None:
        sys.exit(f"spotter-retrain: {key} is required")
    return value


def _image_set(images_dir: Path, paths: tuple[str, ...]) -> ImageSet:
    labels = torch.tensor([CLASSES.index(p.split("/", 1)[0]) for p in paths], dtype=torch.int64)
    return ImageSet(data.load_images(images_dir, list(paths)), labels)


def _get(s3, uri: str) -> bytes:
    bucket, key = uri.removeprefix("s3://").split("/", 1)
    return s3.get_object(Bucket=bucket, Key=key)["Body"].read()


def newest_pytorch(versions) -> pb.ModelVersion:
    torch_versions = [v for v in versions if v.framework == "pytorch"]
    if not torch_versions:
        sys.exit("spotter-retrain: the model has no PyTorch version to start from")
    return max(torch_versions, key=lambda v: v.version)


def describe(event: mpb.DriftDetected | None) -> str:
    if event is None:
        return "started by hand"
    feats = ", ".join(f"{f.feature} PSI {f.psi:.2f}" for f in event.features)
    return f"drift detected on version {event.version} ({feats})"


def run_once(event: mpb.DriftDetected | None = None) -> dict:
    commit = code_commit()
    model_name = _env("MODEL_NAME", "eurosat-classifier")
    floor = float(_env("MIN_ACCURACY", "0.85"))
    p = float(_env("DEGRADE_P", "0.5"))
    s3 = boto3.client(
        "s3",
        endpoint_url=_env("S3_ENDPOINT"),
        aws_access_key_id=_env("S3_ACCESS_KEY"),
        aws_secret_access_key=_env("S3_SECRET_KEY"),
        region_name=_env("S3_REGION", "us-east-1"),
    )
    with grpc.insecure_channel(_env("REGISTRY_ADDR", "localhost:9090")) as channel:
        stub = rpc.RegistryServiceStub(channel)
        versions = stub.ListVersions(
            pb.ListVersionsRequest(model_name=model_name), timeout=10
        ).versions
        parent = newest_pytorch(versions)
        parent_meta = json.loads(_get(s3, parent.artifact_uri.rsplit("/", 1)[0] + "/metadata.json"))
        hp = Hyperparams(
            epochs=int(_env("EPOCHS", "3")),
            learning_rate=float(_env("LEARNING_RATE", "5e-4")),
            seed=int(parent_meta["seed"]),
        )

        images_dir = data.download(Path(_env("DATA_DIR", "data")))
        s = split(data.list_images(images_dir), seed=hp.seed)
        if s.fingerprint() != parent_meta["split_fingerprint"]:
            sys.exit("spotter-retrain: split differs from the parent's; holdout would not match")

        model = EuroSATNet()
        model.load_state_dict(
            torch.load(io.BytesIO(_get(s3, parent.artifact_uri)), weights_only=True)
        )
        parent_model = copy.deepcopy(model)
        log.info(
            "fine-tuning version %d for %d epochs (%s)", parent.version, hp.epochs, describe(event)
        )

        train_set, val_set = _image_set(images_dir, s.train), _image_set(images_dir, s.val)
        result = train(
            model,
            train_set,
            val_set,
            hp,
            on_epoch_end=lambda r: log.info(
                "%s", json.dumps({k: round(v, 4) for k, v in r.items()})
            ),
            augment=make_augment(p),
        )
        # The holdout set is decoded only now, after training.
        holdout = _image_set(images_dir, s.holdout)
        labels = holdout.labels.numpy()

        def predictor(net) -> Callable[[torch.Tensor], np.ndarray]:
            return lambda images: predict(
                net, ImageSet(images, torch.zeros(len(images), dtype=torch.int64))
            )

        new_metrics = robustness(predictor(model), holdout.images, labels)
        parent_metrics = robustness(predictor(parent_model), holdout.images, labels)
        metrics = new_metrics | {"val_accuracy": round(result.best_val_accuracy, 4)}
        trigger = describe(event)
        metadata = {
            "split_fingerprint": s.fingerprint(),
            "seed": hp.seed,
            "classes": list(CLASSES),
            "hyperparams": hp.as_dict() | {"degrade_p": p, "best_epoch": result.best_epoch},
            "parent_version": parent.version,
            "parent_version_id": parent.id,
            "parent_metrics": parent_metrics,
            "trigger": {"description": trigger}
            | (
                {
                    "drifted_version": event.version,
                    "drifted_version_id": event.version_id,
                    "features": [f.feature for f in event.features],
                }
                if event
                else {}
            ),
            "training_seconds": round(result.seconds, 1),
            "code_commit": commit,
        }
        outcome = register_retrained(
            s3,
            _env("S3_BUCKET", "models"),
            stub,
            model_name,
            artifact_bytes(model),
            metrics,
            metadata,
            floor,
            trigger,
        )
    ref = build_reference(torch_predictor(model), val_set.images, outcome.version.id)
    write_reference(s3, outcome.version.artifact_uri, ref)
    summary = {
        "version": outcome.version.version,
        "stage": outcome.stage,
        "parent": parent.version,
        "metrics": metrics,
        "parent_metrics": parent_metrics,
        "training_seconds": round(result.seconds, 1),
    }
    print(json.dumps(summary), flush=True)
    return summary


async def watch(
    js,
    model: str,
    run: Callable[[mpb.DriftDetected], object],
    stop: asyncio.Event,
    ack_wait: float = 3600,
    retry_delay: float = 60,
) -> None:
    """One run per DriftDetected on the model, one at a time; ack after the run, nak on error."""
    sub = await js.pull_subscribe(
        f"monitor.drift.detected.{model}",
        durable=f"retrainer-{model}",
        stream="MONITOR",
        config=ConsumerConfig(
            max_ack_pending=1, ack_wait=ack_wait, deliver_policy=DeliverPolicy.NEW
        ),
    )
    while not stop.is_set():
        try:
            msgs = await sub.fetch(1, timeout=1)
        except TimeoutError:
            continue
        for msg in msgs:
            event = mpb.DriftDetected()
            event.ParseFromString(msg.data)
            try:
                await asyncio.to_thread(run, event)
            except Exception:  # noqa: BLE001 - any failure means a later retry
                log.exception("retraining failed; the drift event will be delivered again")
                await msg.nak(delay=retry_delay)
                continue
            await msg.ack()


async def _watch_main() -> None:
    nc = await nats.connect(_env("NATS_URL"))
    stop = asyncio.Event()
    if sys.platform != "win32":
        for sig in (signal.SIGINT, signal.SIGTERM):
            asyncio.get_running_loop().add_signal_handler(sig, stop.set)
    model = _env("MODEL_NAME", "eurosat-classifier")
    log.info("waiting for drift on %s", model)
    try:
        await watch(nc.jetstream(), model, run_once, stop)
    finally:
        await nc.drain()


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    p = argparse.ArgumentParser(description="Retrain a model version for drifted inputs.")
    p.add_argument("--watch", action="store_true", help="retrain once per DriftDetected event")
    if p.parse_args().watch:
        asyncio.run(_watch_main())
    else:
        run_once()


if __name__ == "__main__":
    main()
