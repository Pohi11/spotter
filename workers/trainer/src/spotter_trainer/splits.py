"""Deterministic, stratified train/validation/holdout split (design D3)."""

import hashlib
import json
import random
from dataclasses import dataclass

TRAIN_FRACTION = 0.70
VAL_FRACTION = 0.15  # holdout gets the rest


@dataclass(frozen=True)
class Split:
    train: tuple[str, ...]
    val: tuple[str, ...]
    holdout: tuple[str, ...]

    def manifest(self) -> dict[str, list[str]]:
        return {
            "train": sorted(self.train),
            "val": sorted(self.val),
            "holdout": sorted(self.holdout),
        }

    def fingerprint(self) -> str:
        """SHA-256 of the canonical JSON manifest: changes if any image changes set."""
        canonical = json.dumps(self.manifest(), sort_keys=True, separators=(",", ":"))
        return hashlib.sha256(canonical.encode()).hexdigest()


def split(paths_by_class: dict[str, list[str]], seed: int) -> Split:
    """Split each class 70/15/15. Depends only on the set of paths and the seed."""
    train: list[str] = []
    val: list[str] = []
    holdout: list[str] = []
    for name in sorted(paths_by_class):
        paths = sorted(paths_by_class[name])  # input order must not matter
        # A per-class generator, so adding a class doesn't reshuffle the others.
        random.Random(f"{seed}:{name}").shuffle(paths)
        n_train = round(len(paths) * TRAIN_FRACTION)
        n_val = round(len(paths) * VAL_FRACTION)
        train += paths[:n_train]
        val += paths[n_train : n_train + n_val]
        holdout += paths[n_train + n_val :]
    return Split(tuple(train), tuple(val), tuple(holdout))
