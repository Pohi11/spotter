"""`spotter-optimize`: export a registered PyTorch version to ONNX fp32 and int8, check,
measure, report, publish, and register the variants (decision record 0011).

Configuration from environment variables:
  MODEL_NAME (eurosat-classifier), SOURCE_VERSION (default: the production version),
  DATA_DIR (data), SEED (42), MIN_ACCURACY (0.85), MAX_ACCURACY_DROP (0.01),
  BENCH_DIR (benchmarks), REGISTRY_ADDR (localhost:9090),
  S3_ENDPOINT, S3_ACCESS_KEY, S3_SECRET_KEY, S3_REGION (us-east-1), S3_BUCKET (models).
"""

import hashlib
import io
import json
import os
import sys
from datetime import UTC, datetime
from pathlib import Path

import boto3
import grpc
import numpy as np
import torch
from spotter.v1 import registry_events_pb2 as events
from spotter.v1 import registry_pb2 as pb
from spotter.v1 import registry_pb2_grpc as rpc
from spotter_model import CLASSES, EuroSATNet

from spotter_trainer import data
from spotter_trainer.metrics import evaluate
from spotter_trainer.optimize import (
    benchmark,
    environment,
    export_onnx,
    normalized,
    onnx_predictor,
    parity,
    predict_all,
    quantize_int8,
    torch_predictor,
    validate_variant,
)
from spotter_trainer.publish import code_commit, publish
from spotter_trainer.reference import build_reference, write_reference
from spotter_trainer.splits import split

CALIBRATION_TILES = 512


def _env(key: str, default: str | None = None) -> str:
    value = os.environ.get(key, default)
    if value is None:
        sys.exit(f"spotter-optimize: {key} is required")
    return value


def _source_version(stub, model_name: str, number: str | None) -> pb.ModelVersion:
    versions = stub.ListVersions(pb.ListVersionsRequest(model_name=model_name), timeout=10).versions
    for v in versions:
        if (number and v.version == int(number)) or (
            not number and v.stage == events.MODEL_VERSION_STAGE_PRODUCTION
        ):
            if v.framework != "pytorch":
                sys.exit(f"spotter-optimize: version {v.version} is {v.framework}, not pytorch")
            return v
    sys.exit(f"spotter-optimize: no {'version ' + number if number else 'production version'}")


def _get(s3, uri: str) -> bytes:
    bucket, key = uri.removeprefix("s3://").split("/", 1)
    return s3.get_object(Bucket=bucket, Key=key)["Body"].read()


def _accuracy(predict, images: np.ndarray, labels: np.ndarray) -> tuple[float, float]:
    report = evaluate(labels, predict_all(predict, images).argmax(1), len(CLASSES))
    return round(report.accuracy, 4), round(report.macro_f1, 4)


def _markdown(report: dict) -> str:
    env = report["environment"]
    lines = [
        f"# Model variants benchmark, {report['date']}",
        "",
        f"Source: `{report['model']}` version {report['source_version']}. "
        f"Holdout: {report['holdout_tiles']} tiles (split `{report['split_fingerprint'][:12]}`).",
        f"Conditions: {env['cpu']}, {env['logical_cores']} logical cores, "
        f"{env['threads']} threads, "
        f"Python {env['python']}, PyTorch {env['torch']}, ONNX Runtime {env['onnxruntime']}.",
        "",
        "| Variant | Size (KB) | Holdout accuracy | Macro F1 |",
        "|---|---|---|---|",
    ]
    for v in report["variants"]:
        lines.append(
            f"| {v['name']} | {v['size_bytes'] / 1024:.0f} | {v['accuracy']:.4f} | "
            f"{v['macro_f1']:.4f} |"
        )
    lines += [
        "",
        "| Variant | Batch | p50 ms | p95 ms | p99 ms | Images/s |",
        "|---|---|---|---|---|---|",
    ]
    for v in report["variants"]:
        for r in v["latency"]:
            lines.append(
                f"| {v['name']} | {r['batch']} | {r['p50_ms']:.2f} | {r['p95_ms']:.2f} | "
                f"{r['p99_ms']:.2f} | {r['images_per_s']:.0f} |"
            )
    p = report["parity"]
    lines += [
        "",
        f"ONNX fp32 parity with PyTorch: max probability difference "
        f"{p['max_abs_diff']:.2e}, {p['disagreements']} disagreeing predictions.",
        "",
    ]
    return "\n".join(lines)


def main() -> None:
    commit = code_commit()
    model_name = _env("MODEL_NAME", "eurosat-classifier")
    floor = float(_env("MIN_ACCURACY", "0.85"))
    tolerance = float(_env("MAX_ACCURACY_DROP", "0.01"))
    seed = int(_env("SEED", "42"))
    bucket = _env("S3_BUCKET", "models")
    s3 = boto3.client(
        "s3",
        endpoint_url=_env("S3_ENDPOINT"),
        aws_access_key_id=_env("S3_ACCESS_KEY"),
        aws_secret_access_key=_env("S3_SECRET_KEY"),
        region_name=_env("S3_REGION", "us-east-1"),
    )
    channel = grpc.insecure_channel(_env("REGISTRY_ADDR", "localhost:9090"))
    stub = rpc.RegistryServiceStub(channel)

    source = _source_version(stub, model_name, os.environ.get("SOURCE_VERSION"))
    artifact = _get(s3, source.artifact_uri)
    if hashlib.sha256(artifact).hexdigest() != source.artifact_sha256:
        sys.exit("spotter-optimize: source artifact checksum mismatch")
    parent_meta = json.loads(_get(s3, source.artifact_uri.rsplit("/", 1)[0] + "/metadata.json"))
    net = EuroSATNet()
    net.load_state_dict(torch.load(io.BytesIO(artifact), weights_only=True))
    net.eval()

    images_dir = data.download(Path(_env("DATA_DIR", "data")))
    s = split(data.list_images(images_dir), seed=seed)
    if s.fingerprint() != parent_meta["split_fingerprint"]:
        sys.exit("spotter-optimize: split differs from the parent's; holdout would not match")
    holdout = normalized(data.load_images(images_dir, list(s.holdout)))
    labels = np.array([CLASSES.index(p.split("/", 1)[0]) for p in s.holdout])
    calibration = normalized(data.load_images(images_dir, list(s.train[:CALIBRATION_TILES])))
    val_images = data.load_images(images_dir, list(s.val))
    print(f"source version {source.version}; holdout {len(labels)} tiles", flush=True)

    torch.set_num_threads(4)
    fp32 = export_onnx(net)
    check = parity(torch_predictor(net), onnx_predictor(fp32), holdout)
    print(
        f"parity: max diff {check.max_abs_diff:.2e}, disagreements {check.disagreements}",
        flush=True,
    )
    if not check.ok:
        sys.exit("spotter-optimize: ONNX export does not match PyTorch; nothing published")
    int8 = quantize_int8(fp32, calibration)

    variants = []
    for name, size, predict in (
        ("pytorch-fp32", len(artifact), torch_predictor(net)),
        ("onnx-fp32", len(fp32), onnx_predictor(fp32)),
        ("onnx-int8", len(int8), onnx_predictor(int8)),
    ):
        acc, f1 = _accuracy(predict, holdout, labels)
        print(f"{name}: accuracy {acc}, benchmarking...", flush=True)
        variants.append(
            {
                "name": name,
                "size_bytes": size,
                "accuracy": acc,
                "macro_f1": f1,
                "latency": benchmark(predict, holdout),
            }
        )

    date = datetime.now(UTC).strftime("%Y-%m-%d")
    report = {
        "date": date,
        "model": model_name,
        "source_version": source.version,
        "source_version_id": source.id,
        "split_fingerprint": s.fingerprint(),
        "holdout_tiles": len(labels),
        "code_commit": commit,
        "environment": environment(),
        "parity": {"max_abs_diff": check.max_abs_diff, "disagreements": check.disagreements},
        "variants": variants,
    }
    out = Path(_env("BENCH_DIR", "benchmarks"))
    out.mkdir(parents=True, exist_ok=True)
    (out / f"{date}-model-variants.json").write_text(json.dumps(report, indent=2))
    (out / f"{date}-model-variants.md").write_text(_markdown(report))
    print(f"report: {out / f'{date}-model-variants.md'}", flush=True)

    parent_acc = variants[0]["accuracy"]
    for variant, model_bytes, optimization in (
        (variants[1], fp32, "fp32"),
        (variants[2], int8, "int8-static"),
    ):
        version, ok, reason = register_variant(
            s3,
            bucket,
            stub,
            model_name,
            model_bytes,
            optimization,
            variant,
            source,
            parent_meta,
            parent_acc,
            tolerance,
            floor,
            commit,
        )
        ref = build_reference(onnx_predictor(model_bytes), val_images, version.id)
        write_reference(s3, version.artifact_uri, ref)
        print(
            f"registered {variant['name']} as version {version.version}: "
            f"{'validated' if ok else 'registered'} ({reason})",
            flush=True,
        )
    channel.close()


def register_variant(
    s3,
    bucket: str,
    stub,
    model_name: str,
    model_bytes: bytes,
    optimization: str,
    variant: dict,
    source: pb.ModelVersion,
    parent_meta: dict,
    parent_accuracy: float,
    tolerance: float,
    floor: float,
    commit: str,
) -> tuple[pb.ModelVersion, bool, str]:
    """Publish one variant with lineage metadata, register it, and validate it if it passes."""
    lat1 = next(r for r in variant["latency"] if r["batch"] == 1)
    metrics = {
        "accuracy": variant["accuracy"],
        "macro_f1": variant["macro_f1"],
        "p50_ms_batch1": round(lat1["p50_ms"], 3),
    }
    meta = {
        **{k: parent_meta[k] for k in ("split_fingerprint", "seed", "classes")},
        "parent_version": source.version,
        "parent_version_id": source.id,
        "optimization": optimization,
        "metrics": metrics,
        "code_commit": commit,
        "environment": environment(),
    }
    published = publish(s3, bucket, model_name, model_bytes, meta)
    version = stub.RegisterVersion(
        pb.RegisterVersionRequest(
            model_name=model_name,
            artifact_uri=published.uri,
            artifact_sha256=published.sha256,
            framework="onnx",
            metrics=metrics,
        ),
        timeout=10,
    ).version
    ok, reason = validate_variant(parent_accuracy, variant["accuracy"], tolerance, floor)
    if ok:
        stub.TransitionVersion(
            pb.TransitionVersionRequest(
                version_id=version.id,
                to_stage=events.MODEL_VERSION_STAGE_VALIDATED,
                reason=reason,
                expected_revision=version.revision,
            ),
            timeout=10,
        )
    return version, ok, reason


if __name__ == "__main__":
    main()
