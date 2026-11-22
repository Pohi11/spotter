import asyncio
import os
import uuid

import nats
import pytest
from spotter.v1 import monitor_pb2 as mpb
from spotter_trainer.retrain_cli import watch

WATCH = ("model-retraining", "Each drift event triggers one retraining")


@pytest.fixture
async def js():
    if not os.environ.get("NATS_URL"):
        pytest.skip("NATS_URL not set")
    nc = await nats.connect(os.environ["NATS_URL"])
    js = nc.jetstream()
    await js.stream_info("MONITOR")  # created by `spotterctl setup`
    yield js
    await nc.close()


async def drift(js, model: str, version: int) -> None:
    ev = mpb.DriftDetected(model_name=model, version_id=f"v{version}", version=version)
    await js.publish(
        f"monitor.drift.detected.{model}",
        ev.SerializeToString(),
        headers={"Nats-Msg-Id": uuid.uuid4().hex, "Spotter-Event-Type": "spotter.v1.DriftDetected"},
    )


async def until(cond, seconds=10.0):
    for _ in range(int(seconds * 20)):
        if cond():
            return
        await asyncio.sleep(0.05)
    raise AssertionError("condition not met in time")


@pytest.mark.integration
@pytest.mark.spec(*WATCH)
async def test_one_run_per_event_in_order(js):
    model = f"m-{uuid.uuid4().hex[:10]}"
    runs, active = [], []

    def run(event):
        active.append(1)
        assert len(active) == 1, "two runs at once"
        runs.append(event.version)
        import time

        time.sleep(0.2)
        active.pop()

    stop = asyncio.Event()
    task = asyncio.create_task(watch(js, model, run, stop))
    await asyncio.sleep(0.5)  # the consumer delivers only events published after it exists
    await drift(js, model, 3)
    await drift(js, model, 4)
    await until(lambda: len(runs) == 2)
    stop.set()
    await task
    assert runs == [3, 4]
    await js.delete_consumer("MONITOR", f"retrainer-{model}")


@pytest.mark.integration
@pytest.mark.spec(*WATCH)
async def test_failed_run_is_delivered_again(js):
    model = f"m-{uuid.uuid4().hex[:10]}"
    calls = []

    def run(event):
        calls.append(event.version)
        if len(calls) == 1:
            raise RuntimeError("out of memory")

    stop = asyncio.Event()
    task = asyncio.create_task(watch(js, model, run, stop, retry_delay=0.1))
    await asyncio.sleep(0.5)
    await drift(js, model, 3)
    await until(lambda: len(calls) == 2)
    stop.set()
    await task
    assert calls == [3, 3]
    info = await js.consumer_info("MONITOR", f"retrainer-{model}")
    assert info.num_ack_pending == 0
    await js.delete_consumer("MONITOR", f"retrainer-{model}")
