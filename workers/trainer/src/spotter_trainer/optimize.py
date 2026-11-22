"""Export, check, quantize, evaluate, and benchmark model variants (decision record 0011)."""

import contextlib
import io
import platform
import tempfile
import time
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path

import numpy as np
import onnxruntime as ort
import torch
from onnxruntime.quantization import (
    CalibrationDataReader,
    QuantFormat,
    QuantType,
    quantize_static,
)
from onnxruntime.quantization.shape_inference import quant_pre_process
from spotter_model import normalize
from torch import nn

PARITY_TOLERANCE = 1e-4
THREADS = 4

Predict = Callable[[np.ndarray], np.ndarray]  # normalized float32 [N,3,64,64] -> probs [N,10]


# --- pure functions -------------------------------------------------------------------


def percentiles(samples_ms: list[float]) -> dict[str, float]:
    """p50, p95, p99 with linear interpolation (numpy's default)."""
    arr = np.asarray(samples_ms, dtype=np.float64)
    p50, p95, p99 = np.percentile(arr, [50, 95, 99])
    return {"p50_ms": float(p50), "p95_ms": float(p95), "p99_ms": float(p99)}


def validate_variant(
    parent_accuracy: float, variant_accuracy: float, tolerance: float, floor: float
) -> tuple[bool, str]:
    """A variant is validated only within tolerance of its parent and at or above the floor."""
    ok = variant_accuracy >= parent_accuracy - tolerance and variant_accuracy >= floor
    verdict = "within" if ok else "outside"
    reason = (
        f"holdout accuracy {variant_accuracy:.4f} vs parent {parent_accuracy:.4f}: "
        f"{verdict} tolerance {tolerance:.2f} (floor {floor:.2f})"
    )
    return ok, reason


def softmax(logits: np.ndarray) -> np.ndarray:
    z = logits - logits.max(axis=1, keepdims=True)
    e = np.exp(z)
    return e / e.sum(axis=1, keepdims=True)


# --- runtimes -------------------------------------------------------------------------


def torch_predictor(net: nn.Module) -> Predict:
    net.eval()

    def predict(x: np.ndarray) -> np.ndarray:
        with torch.no_grad():
            return torch.softmax(net(torch.from_numpy(x)), dim=1).numpy()

    return predict


def onnx_session(model: bytes, threads: int = THREADS) -> ort.InferenceSession:
    opts = ort.SessionOptions()
    opts.intra_op_num_threads = threads
    opts.inter_op_num_threads = 1
    opts.graph_optimization_level = ort.GraphOptimizationLevel.ORT_ENABLE_ALL
    return ort.InferenceSession(model, sess_options=opts, providers=["CPUExecutionProvider"])


def onnx_predictor(model: bytes, threads: int = THREADS) -> Predict:
    session = onnx_session(model, threads)
    name = session.get_inputs()[0].name

    def predict(x: np.ndarray) -> np.ndarray:
        return softmax(session.run(None, {name: x})[0])

    return predict


def predict_all(predict: Predict, images: np.ndarray, batch: int = 256) -> np.ndarray:
    return np.concatenate([predict(images[i : i + batch]) for i in range(0, len(images), batch)])


def normalized(images_uint8: torch.Tensor) -> np.ndarray:
    return normalize(images_uint8).numpy().astype(np.float32)


# --- export and quantization ----------------------------------------------------------


def export_onnx(net: nn.Module) -> bytes:
    """Export to one self-contained ONNX file with a dynamic batch dimension."""
    net.eval()
    # The exporter prints progress (with emoji some consoles cannot encode); keep it quiet.
    with tempfile.TemporaryDirectory() as d, contextlib.redirect_stdout(io.StringIO()):
        path = Path(d) / "model.onnx"
        torch.onnx.export(
            net,
            (torch.zeros(2, 3, 64, 64),),
            path,
            input_names=["input"],
            output_names=["logits"],
            dynamic_shapes={"x": {0: torch.export.Dim("batch")}},
            dynamo=True,
            external_data=False,  # weights inside the file: one artifact, one checksum
            verbose=False,
        )
        return path.read_bytes()


@dataclass(frozen=True)
class Parity:
    max_abs_diff: float
    disagreements: int

    @property
    def ok(self) -> bool:
        return self.max_abs_diff < PARITY_TOLERANCE and self.disagreements == 0


def parity(reference: Predict, candidate: Predict, images: np.ndarray) -> Parity:
    a, b = predict_all(reference, images), predict_all(candidate, images)
    return Parity(
        max_abs_diff=float(np.abs(a - b).max()),
        disagreements=int((a.argmax(1) != b.argmax(1)).sum()),
    )


class _Calibration(CalibrationDataReader):
    def __init__(self, images: np.ndarray, batch: int = 32) -> None:
        self._batches = iter(
            [{"input": images[i : i + batch]} for i in range(0, len(images), batch)]
        )

    def get_next(self):
        return next(self._batches, None)


def quantize_int8(fp32_model: bytes, calibration: np.ndarray) -> bytes:
    """Static int8 quantization: QDQ format, per-channel weights, calibrated on `calibration`."""
    with tempfile.TemporaryDirectory() as d:
        src, pre, out = Path(d) / "fp32.onnx", Path(d) / "pre.onnx", Path(d) / "int8.onnx"
        src.write_bytes(fp32_model)
        quant_pre_process(str(src), str(pre))
        quantize_static(
            str(pre),
            str(out),
            _Calibration(calibration),
            quant_format=QuantFormat.QDQ,
            per_channel=True,
            activation_type=QuantType.QInt8,
            weight_type=QuantType.QInt8,
        )
        return out.read_bytes()


# --- benchmarking ---------------------------------------------------------------------

ITERATIONS = {1: 200, 8: 100, 32: 50}
WARMUP = 10


def benchmark(
    predict: Predict, images: np.ndarray, batch_sizes=(1, 8, 32), iterations=None
) -> list[dict]:
    """Latency per batch after warm-up, on real images, cycling through `images`."""
    iterations = iterations or ITERATIONS
    rows = []
    for b in batch_sizes:
        batches = [images[(i * b) % (len(images) - b) :][:b] for i in range(WARMUP + iterations[b])]
        for x in batches[:WARMUP]:
            predict(x)
        samples = []
        for x in batches[WARMUP:]:
            start = time.perf_counter()
            predict(x)
            samples.append((time.perf_counter() - start) * 1000)
        p = percentiles(samples)
        rows.append(
            {
                "batch": b,
                "iterations": len(samples),
                **p,
                "images_per_s": b * 1000 / float(np.mean(samples)),
            }
        )
    return rows


def environment() -> dict:
    import onnxruntime

    return {
        "cpu": platform.processor() or platform.machine(),
        "logical_cores": __import__("os").cpu_count(),
        "threads": THREADS,
        "python": platform.python_version(),
        "torch": torch.__version__,
        "onnxruntime": onnxruntime.__version__,
        "platform": platform.platform(),
    }
