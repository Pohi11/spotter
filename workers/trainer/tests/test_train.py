import pytest
import torch
from spotter_model import EuroSATNet
from spotter_trainer.train import Hyperparams, ImageSet, evaluate_holdout, train

HOLDOUT = ("model-evaluation", "The holdout set is used only for final evaluation")


def synthetic(n: int, seed: int) -> ImageSet:
    """Two easy classes: dark tiles (0) and bright tiles (1)."""
    g = torch.Generator().manual_seed(seed)
    labels = torch.randint(0, 2, (n,), generator=g)
    base = torch.where(labels.view(-1, 1, 1, 1) == 1, 200, 40)
    noise = torch.randint(0, 30, (n, 3, 64, 64), generator=g)
    return ImageSet((base + noise).to(torch.uint8), labels)


class RecordingSet(ImageSet):
    """A holdout set that logs every read into a shared event list."""

    def __init__(self, inner: ImageSet, events: list[str]) -> None:
        super().__init__(inner.images, inner.labels)
        self.events = events

    def batch(self, idx):
        self.events.append("holdout read")
        return super().batch(idx)

    @property
    def labels(self):
        self.events.append("holdout read")
        return self._labels

    @labels.setter
    def labels(self, value):
        self._labels = value


@pytest.mark.spec(*HOLDOUT)
def test_holdout_untouched_until_training_ends():
    events: list[str] = []
    holdout = RecordingSet(synthetic(64, seed=3), events)
    events.clear()  # constructing the set isn't a read

    model = EuroSATNet(num_classes=2)
    train(
        model,
        synthetic(128, seed=1),
        synthetic(64, seed=2),
        Hyperparams(epochs=2, batch_size=32),
        on_epoch_end=lambda _: events.append("epoch end"),
    )
    report = evaluate_holdout(model, holdout, num_classes=2)

    assert events.count("epoch end") == 2
    first_read = events.index("holdout read")
    assert all(e == "holdout read" for e in events[first_read:]), events
    assert events[first_read - 1] == "epoch end"  # the last epoch finished first
    # The published metrics come from the holdout set.
    assert report.confusion.sum() == len(holdout._labels)


def test_training_learns_an_easy_task_and_keeps_the_best_epoch():
    model = EuroSATNet(num_classes=2)
    result = train(
        model, synthetic(256, seed=1), synthetic(128, seed=2), Hyperparams(epochs=3, batch_size=32)
    )
    assert result.best_val_accuracy >= 0.95
    assert result.best_epoch == max(result.history, key=lambda r: r["val_accuracy"])["epoch"]
    assert evaluate_holdout(model, synthetic(128, seed=3), num_classes=2).accuracy >= 0.95
