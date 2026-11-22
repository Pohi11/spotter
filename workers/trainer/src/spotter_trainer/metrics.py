"""Classification metrics. Pure numpy, no I/O, so every number can be checked by hand."""

from dataclasses import dataclass

import numpy as np


@dataclass(frozen=True)
class Report:
    confusion: np.ndarray  # [true, predicted]
    accuracy: float
    precision: np.ndarray  # per class
    recall: np.ndarray  # per class
    f1: np.ndarray  # per class
    macro_f1: float


def confusion_matrix(y_true: np.ndarray, y_pred: np.ndarray, num_classes: int) -> np.ndarray:
    """Count of (true class, predicted class) pairs: rows are true, columns are predicted."""
    cm = np.zeros((num_classes, num_classes), dtype=np.int64)
    np.add.at(cm, (np.asarray(y_true), np.asarray(y_pred)), 1)
    return cm


def _safe_divide(num: np.ndarray, den: np.ndarray) -> np.ndarray:
    """num / den, with 0 wherever den is 0 (a class never predicted, or never present)."""
    out = np.zeros_like(num, dtype=np.float64)
    np.divide(num, den, out=out, where=den != 0)
    return out


def evaluate(y_true: np.ndarray, y_pred: np.ndarray, num_classes: int) -> Report:
    cm = confusion_matrix(y_true, y_pred, num_classes)
    tp = np.diag(cm).astype(np.float64)
    predicted = cm.sum(axis=0).astype(np.float64)  # column sums
    actual = cm.sum(axis=1).astype(np.float64)  # row sums
    precision = _safe_divide(tp, predicted)
    recall = _safe_divide(tp, actual)
    f1 = _safe_divide(2 * precision * recall, precision + recall)
    total = cm.sum()
    return Report(
        confusion=cm,
        accuracy=float(tp.sum() / total) if total else 0.0,
        precision=precision,
        recall=recall,
        f1=f1,
        macro_f1=float(f1.mean()),
    )
