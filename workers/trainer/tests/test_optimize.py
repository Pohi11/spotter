import numpy as np
import pytest
import torch
from spotter_model import EuroSATNet
from spotter_trainer.optimize import (
    benchmark,
    export_onnx,
    normalized,
    onnx_predictor,
    parity,
    percentiles,
    quantize_int8,
    torch_predictor,
    validate_variant,
)

EXPORT = ("model-optimization", "The ONNX export matches PyTorch")
TOLERANCE = (
    "model-optimization",
    "Quantized versions are validated only within an accuracy tolerance",
)
BENCH = ("model-optimization", "Benchmarks report percentiles, throughput, and their conditions")


@pytest.fixture(scope="module")
def images() -> np.ndarray:
    g = torch.Generator().manual_seed(0)
    return normalized(torch.randint(0, 256, (64, 3, 64, 64), generator=g, dtype=torch.uint8))


@pytest.fixture(scope="module")
def net() -> EuroSATNet:
    torch.manual_seed(0)
    return EuroSATNet().eval()


@pytest.mark.spec(*EXPORT)
def test_faithful_export(net, images):
    p = parity(torch_predictor(net), onnx_predictor(export_onnx(net)), images)
    assert p.ok, p
    assert p.max_abs_diff < 1e-4 and p.disagreements == 0


@pytest.mark.spec(*EXPORT)
def test_unfaithful_export_is_rejected(net, images):
    other = EuroSATNet().eval()  # different random weights: a wrong export
    p = parity(torch_predictor(net), onnx_predictor(export_onnx(other)), images)
    assert not p.ok


@pytest.mark.spec(*EXPORT)
def test_export_is_one_self_contained_file(net, images):
    model = export_onnx(net)
    assert len(model) > 1_000_000  # 288,746 float32 weights are inside, not in a side file
    out = onnx_predictor(model)(images[:5])
    assert out.shape == (5, 10)


def test_int8_model_runs_and_is_smaller(net, images):
    fp32 = export_onnx(net)
    int8 = quantize_int8(fp32, images[:32])
    assert len(int8) < len(fp32) / 2
    assert onnx_predictor(int8)(images[:4]).shape == (4, 10)


@pytest.mark.spec(*TOLERANCE)
def test_within_tolerance():
    ok, reason = validate_variant(0.9657, 0.9640, tolerance=0.01, floor=0.85)
    assert ok and "0.9640" in reason and "0.9657" in reason


@pytest.mark.spec(*TOLERANCE)
def test_too_much_accuracy_lost():
    ok, reason = validate_variant(0.9657, 0.9400, tolerance=0.01, floor=0.85)
    assert not ok and "0.9400" in reason and "0.9657" in reason


@pytest.mark.spec(*TOLERANCE)
def test_below_the_floor_even_within_tolerance():
    ok, _ = validate_variant(0.855, 0.848, tolerance=0.01, floor=0.85)
    assert not ok


@pytest.mark.spec(*BENCH)
def test_percentiles_from_samples():
    p = percentiles([float(i) for i in range(1, 101)])
    assert p["p50_ms"] == pytest.approx(50.5)
    assert p["p95_ms"] == pytest.approx(95.05)
    assert p["p99_ms"] == pytest.approx(99.01)


@pytest.mark.spec(*BENCH)
def test_benchmark_rows(net, images):
    rows = benchmark(torch_predictor(net), images, batch_sizes=(1, 8), iterations={1: 5, 8: 3})
    assert [r["batch"] for r in rows] == [1, 8]
    for r in rows:
        assert r["p50_ms"] <= r["p95_ms"] <= r["p99_ms"]
        assert r["images_per_s"] > 0 and r["iterations"] in (5, 3)
