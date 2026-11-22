import numpy as np
import pytest
from spotter_trainer.metrics import evaluate

METRICS = ("model-evaluation", "Evaluation reports per-class metrics")


@pytest.mark.spec(*METRICS)
def test_known_predictions():
    r = evaluate(np.array([0, 0, 1, 1]), np.array([0, 1, 1, 1]), num_classes=2)
    assert r.confusion.tolist() == [[1, 1], [0, 2]]
    assert r.accuracy == pytest.approx(0.75)
    assert r.precision == pytest.approx([1.0, 2 / 3])
    assert r.recall == pytest.approx([0.5, 1.0])
    # F1: class 0 = 2*1*0.5/1.5 = 2/3; class 1 = 2*(2/3)*1/(5/3) = 0.8
    assert r.f1 == pytest.approx([2 / 3, 0.8])
    assert r.macro_f1 == pytest.approx((2 / 3 + 0.8) / 2)


@pytest.mark.spec(*METRICS)
def test_class_never_predicted_is_zero_not_nan():
    r = evaluate(np.array([0, 1, 2]), np.array([0, 0, 0]), num_classes=3)
    assert r.precision[1] == 0 and r.precision[2] == 0
    assert r.recall[1] == 0 and r.recall[2] == 0
    for value in [*r.precision, *r.recall, *r.f1, r.accuracy, r.macro_f1]:
        assert not np.isnan(value)


@pytest.mark.spec(*METRICS)
def test_class_with_no_true_examples_has_zero_recall():
    r = evaluate(np.array([0, 0]), np.array([0, 1]), num_classes=2)
    assert r.recall[1] == 0 and not np.isnan(r.recall[1])


@pytest.mark.spec(*METRICS)
def test_perfect_predictions():
    y = np.array([0, 1, 2, 2, 1, 0])
    r = evaluate(y, y, num_classes=3)
    assert r.accuracy == 1.0 and r.macro_f1 == 1.0
    assert (r.precision == 1.0).all() and (r.recall == 1.0).all()
