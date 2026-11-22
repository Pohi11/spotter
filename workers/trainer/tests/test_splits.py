import random

import pytest
from spotter_trainer.splits import split

DETERMINISTIC = ("training-data", "Splits are deterministic")
STRATIFIED = ("training-data", "Splits are stratified and disjoint")


def dataset(sizes: dict[str, int]) -> dict[str, list[str]]:
    return {c: [f"{c}/{c}_{i}.jpg" for i in range(1, n + 1)] for c, n in sizes.items()}


DATA = dataset({"Forest": 3000, "River": 2500, "Highway": 2501})


@pytest.mark.spec(*DETERMINISTIC)
def test_same_seed_same_split():
    a, b = split(DATA, seed=42), split(DATA, seed=42)
    assert a == b
    assert a.fingerprint() == b.fingerprint()


@pytest.mark.spec(*DETERMINISTIC)
def test_different_seed_different_split():
    a, b = split(DATA, seed=42), split(DATA, seed=7)
    assert a.manifest() != b.manifest()
    assert a.fingerprint() != b.fingerprint()


@pytest.mark.spec(*DETERMINISTIC)
def test_input_order_does_not_matter():
    shuffled = {c: random.Random(1).sample(paths, len(paths)) for c, paths in DATA.items()}
    assert split(shuffled, seed=42) == split(DATA, seed=42)


@pytest.mark.spec(*DETERMINISTIC)
def test_moving_one_image_changes_the_fingerprint():
    s = split(DATA, seed=42)
    moved = type(s)(train=s.train[1:], val=s.val + s.train[:1], holdout=s.holdout)
    assert moved.fingerprint() != s.fingerprint()


@pytest.mark.spec(*STRATIFIED)
def test_class_proportions():
    s = split(dataset({"Forest": 3000}), seed=42)
    assert (len(s.train), len(s.val), len(s.holdout)) == (2100, 450, 450)


@pytest.mark.spec(*STRATIFIED)
def test_every_class_is_split_70_15_15():
    s = split(DATA, seed=42)
    for cls, paths in DATA.items():
        n = len(paths)
        counts = [
            sum(p.startswith(cls + "/") for p in part) for part in (s.train, s.val, s.holdout)
        ]
        assert sum(counts) == n
        assert abs(counts[0] - 0.70 * n) <= 1 and abs(counts[1] - 0.15 * n) <= 1


@pytest.mark.spec(*STRATIFIED)
def test_no_overlap_and_nothing_missing():
    s = split(DATA, seed=42)
    train, val, holdout = set(s.train), set(s.val), set(s.holdout)
    assert not (train & val or train & holdout or val & holdout)
    everything = {p for paths in DATA.values() for p in paths}
    assert train | val | holdout == everything
    assert len(s.train) + len(s.val) + len(s.holdout) == len(everything)
