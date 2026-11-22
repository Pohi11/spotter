"""Training (train + validation sets only) and holdout evaluation (design D4)."""

import copy
import time
from collections.abc import Callable
from dataclasses import asdict, dataclass, field

import numpy as np
import torch
from spotter_model import normalize
from torch import nn

from spotter_trainer.metrics import Report, evaluate


@dataclass(frozen=True)
class Hyperparams:
    epochs: int = 12
    batch_size: int = 128
    learning_rate: float = 2e-3
    weight_decay: float = 1e-4
    seed: int = 42

    def as_dict(self) -> dict:
        return asdict(self)


class ImageSet:
    """Images as one uint8 tensor [N, 3, 64, 64] plus integer labels [N]."""

    def __init__(self, images: torch.Tensor, labels: torch.Tensor) -> None:
        if len(images) != len(labels):
            raise ValueError("images and labels differ in length")
        self.images = images
        self.labels = labels

    def __len__(self) -> int:
        return len(self.labels)

    def batch(self, idx: torch.Tensor) -> tuple[torch.Tensor, torch.Tensor]:
        return self.images[idx], self.labels[idx]


@dataclass
class TrainResult:
    best_epoch: int
    best_val_accuracy: float
    seconds: float
    history: list[dict] = field(default_factory=list)


def _augment(x: torch.Tensor, g: torch.Generator) -> torch.Tensor:
    """Random horizontal and vertical flips: overhead imagery has no "up"."""
    n = x.shape[0]
    h = torch.rand(n, generator=g) < 0.5
    v = torch.rand(n, generator=g) < 0.5
    x = x.clone()
    x[h] = x[h].flip(-1)
    x[v] = x[v].flip(-2)
    return x


def predict(model: nn.Module, data: ImageSet, batch_size: int = 512) -> np.ndarray:
    model.eval()
    preds = []
    with torch.no_grad():
        for start in range(0, len(data), batch_size):
            x, _ = data.batch(torch.arange(start, min(start + batch_size, len(data))))
            preds.append(model(normalize(x)).argmax(dim=1))
    return torch.cat(preds).numpy() if preds else np.array([], dtype=np.int64)


def train(
    model: nn.Module,
    train_set: ImageSet,
    val_set: ImageSet,
    hp: Hyperparams,
    on_epoch_end: Callable[[dict], None] | None = None,
) -> TrainResult:
    """Train on train_set, choose the best epoch on val_set, and load its weights.

    The holdout set is deliberately not a parameter.
    """
    torch.manual_seed(hp.seed)
    g = torch.Generator().manual_seed(hp.seed)
    opt = torch.optim.AdamW(model.parameters(), lr=hp.learning_rate, weight_decay=hp.weight_decay)
    sched = torch.optim.lr_scheduler.OneCycleLR(
        opt,
        max_lr=hp.learning_rate,
        epochs=hp.epochs,
        steps_per_epoch=max(1, -(-len(train_set) // hp.batch_size)),
    )
    loss_fn = nn.CrossEntropyLoss()
    best_state, best_acc, best_epoch = None, -1.0, 0
    history = []
    start = time.perf_counter()

    for epoch in range(1, hp.epochs + 1):
        model.train()
        perm = torch.randperm(len(train_set), generator=g)
        total_loss = 0.0
        for i in range(0, len(perm), hp.batch_size):
            x, y = train_set.batch(perm[i : i + hp.batch_size])
            logits = model(normalize(_augment(x, g)))
            loss = loss_fn(logits, y)
            opt.zero_grad()
            loss.backward()
            opt.step()
            sched.step()
            total_loss += loss.item() * len(y)

        val_acc = float((predict(model, val_set) == val_set.labels.numpy()).mean())
        record = {
            "epoch": epoch,
            "train_loss": total_loss / len(train_set),
            "val_accuracy": val_acc,
            "seconds": time.perf_counter() - start,
        }
        history.append(record)
        if on_epoch_end:
            on_epoch_end(record)
        if val_acc > best_acc:
            best_acc, best_epoch = val_acc, epoch
            best_state = copy.deepcopy(model.state_dict())

    if best_state is not None:
        model.load_state_dict(best_state)
    return TrainResult(best_epoch, best_acc, time.perf_counter() - start, history)


def evaluate_holdout(model: nn.Module, holdout: ImageSet, num_classes: int) -> Report:
    """Read the holdout set once, after training, for the published metrics."""
    return evaluate(holdout.labels.numpy(), predict(model, holdout), num_classes)
