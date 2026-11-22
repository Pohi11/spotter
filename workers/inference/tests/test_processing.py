"""inference-processing scenarios against real NATS JetStream and SeaweedFS."""

import asyncio

import pytest
from nats.errors import TimeoutError as NatsTimeout
from nats.js.api import DeliverPolicy
from spotter.v1 import inference_pb2 as ipb
from spotter.v1 import registry_events_pb2 as events
from spotter_inference.models import ChecksumMismatch, LoadedModel
from spotter_inference.processor import Processor

from .conftest import jpeg

SERVES = ("inference-processing", "The worker serves its model's production version")
ONE_RESULT = ("inference-processing", "Each job produces exactly one result")
DEAD = ("inference-processing", "Jobs that cannot succeed are dead-lettered")
FOLLOWS = ("inference-processing", "The worker follows production changes")

pytestmark = pytest.mark.integration


class Crash(Exception):
    pass


async def loaded_holder(infra, number=1):
    infra.registry.versions = [infra.publish_version(number)]
    holder = infra.holder()
    await holder.reload()
    return holder


def processor(infra, holder, **kw) -> Processor:
    return Processor(
        js=infra.js,
        kv=infra.kv,
        s3=infra.s3,
        model=lambda: holder.current,
        max_deliver=kw.pop("max_deliver", 5),
        retry_delay=kw.pop("retry_delay", 0.2),
        **kw,
    )


async def fetch_one(sub, wait=5):
    (msg,) = await sub.fetch(1, timeout=wait)
    return msg


@pytest.mark.spec(*SERVES)
@pytest.mark.spec(*ONE_RESULT)
async def test_successful_job(infra):
    holder = await loaded_holder(infra, number=3)
    sub = await infra.consumer()
    job_id = await infra.submit(jpeg())

    await processor(infra, holder).handle(await fetch_one(sub))

    job, _ = await infra.job(job_id)
    assert job.status == ipb.JOB_STATUS_SUCCEEDED
    assert job.prediction.model_version == 3 and job.prediction.model_version_id == "version-3"
    assert len(job.prediction.scores) == 10 and 0 <= job.prediction.confidence <= 1
    completed = await infra.messages(f"inference.completed.{infra.model}")
    assert len(completed) == 1
    event = ipb.InferenceCompleted.FromString(completed[0].data)
    assert event.job_id == job_id and len(event.input_stats.channel_mean) == 3


@pytest.mark.spec(*ONE_RESULT)
async def test_worker_stops_before_acknowledging(infra):
    holder = await loaded_holder(infra)
    sub = await infra.consumer(ack_wait=1.0)
    job_id = await infra.submit(jpeg())

    def crash():
        raise Crash

    with pytest.raises(Crash):  # result recorded and published, then "crash" before ack
        await processor(infra, holder, before_ack=crash).handle(await fetch_one(sub))
    _, revision_after_first = await infra.job(job_id)

    redelivered = await fetch_one(sub, wait=5)  # after the 1 s ack wait
    assert redelivered.metadata.num_delivered == 2
    await processor(infra, holder).handle(redelivered)

    job, revision = await infra.job(job_id)
    assert job.status == ipb.JOB_STATUS_SUCCEEDED
    assert revision == revision_after_first  # no second result recorded
    assert await infra.count(f"inference.completed.{infra.model}") == 1


@pytest.mark.spec(*ONE_RESULT)
async def test_worker_stops_before_recording(infra):
    holder = await loaded_holder(infra)
    sub = await infra.consumer(ack_wait=1.0)
    job_id = await infra.submit(jpeg())

    await fetch_one(sub)  # received, then the worker "dies": no record, no ack
    redelivered = await fetch_one(sub, wait=5)
    await processor(infra, holder).handle(redelivered)

    job, _ = await infra.job(job_id)
    assert job.status == ipb.JOB_STATUS_SUCCEEDED


@pytest.mark.spec(*DEAD)
async def test_corrupt_image_is_dead_lettered(infra):
    holder = await loaded_holder(infra)
    sub = await infra.consumer(ack_wait=1.0)
    job_id = await infra.submit(b"this is not an image")

    await processor(infra, holder).handle(await fetch_one(sub))

    job, _ = await infra.job(job_id)
    assert job.status == ipb.JOB_STATUS_FAILED and "decoded" in job.error
    failed = await infra.messages(f"inference.failed.{infra.model}")
    assert len(failed) == 1 and ipb.InferenceFailed.FromString(failed[0].data).job_id == job_id
    with pytest.raises(NatsTimeout):  # terminated: never delivered again
        await sub.fetch(1, timeout=2)


@pytest.mark.spec(*DEAD)
async def test_undecodable_event_is_dead_lettered(infra):
    holder = await loaded_holder(infra)
    sub = await infra.consumer(ack_wait=1.0)
    await infra.js.publish(f"inference.requested.{infra.model}", b"\xff\xfe not protobuf")

    await processor(infra, holder).handle(await fetch_one(sub))

    assert await infra.count(f"inference.failed.{infra.model}") == 1
    with pytest.raises(NatsTimeout):
        await sub.fetch(1, timeout=2)


@pytest.mark.spec(*SERVES)
@pytest.mark.spec(*DEAD)
async def test_no_model_is_retried_then_dead_lettered(infra):
    holder = infra.holder()  # nothing loaded
    sub = await infra.consumer(ack_wait=5.0, max_deliver=2)
    job_id = await infra.submit(jpeg())
    p = processor(infra, holder, max_deliver=2, retry_delay=0.2)

    await p.handle(await fetch_one(sub))  # first delivery: nak with delay
    job, _ = await infra.job(job_id)
    assert job.status == ipb.JOB_STATUS_PENDING

    second = await fetch_one(sub, wait=5)  # redelivered after the nak delay
    assert second.metadata.num_delivered == 2
    await p.handle(second)  # last allowed delivery: fail and dead-letter

    job, _ = await infra.job(job_id)
    assert job.status == ipb.JOB_STATUS_FAILED and "no production model" in job.error
    assert await infra.count(f"inference.failed.{infra.model}") == 1


@pytest.mark.spec(*SERVES)
async def test_checksum_mismatch_is_not_loaded(infra):
    infra.registry.versions = [infra.publish_version(1, corrupt_checksum=True)]
    holder = infra.holder()
    with pytest.raises(ChecksumMismatch):
        await holder.reload()
    assert holder.current is None


@pytest.mark.spec(*FOLLOWS)
async def test_promotion_while_running(infra):
    holder = await loaded_holder(infra, number=1)
    v2 = infra.publish_version(2)
    retired = infra.registry.versions[0]
    retired.stage = events.MODEL_VERSION_STAGE_RETIRED
    infra.registry.versions = [v2, retired]

    reloaded = asyncio.Event()

    async def on_change(msg):
        if holder.follows(events.ModelVersionStageChanged.FromString(msg.data)):
            await holder.reload()
            reloaded.set()

    await infra.js.subscribe(
        "registry.events.model_version_stage_changed",
        cb=on_change,
        ordered_consumer=True,
        deliver_policy=DeliverPolicy.NEW,
    )
    change = events.ModelVersionStageChanged(
        model_name=infra.model,
        version_id=v2.id,
        version=2,
        from_stage=events.MODEL_VERSION_STAGE_SHADOW,
        to_stage=events.MODEL_VERSION_STAGE_PRODUCTION,
    )
    await infra.js.publish(
        "registry.events.model_version_stage_changed", change.SerializeToString()
    )
    await asyncio.wait_for(reloaded.wait(), timeout=10)

    sub = await infra.consumer()
    job_id = await infra.submit(jpeg())
    await processor(infra, holder).handle(await fetch_one(sub))
    job, _ = await infra.job(job_id)
    assert job.prediction.model_version == 2


@pytest.mark.spec(*SERVES)
async def test_onnx_production_version(infra):
    infra.registry.versions = [infra.publish_version(4, framework="onnx")]
    holder = infra.holder()
    await holder.reload()
    assert holder.current.framework == "onnx"
    sub = await infra.consumer()
    job_id = await infra.submit(jpeg())

    await processor(infra, holder).handle(await fetch_one(sub))

    job, _ = await infra.job(job_id)
    assert job.status == ipb.JOB_STATUS_SUCCEEDED and job.prediction.model_version == 4
    assert abs(sum(s.score for s in job.prediction.scores) - 1) < 1e-4


SHADOW_RUNS = ("shadow-inference", "The worker runs the shadow version without affecting results")
SHADOW_EVENT = ("shadow-inference", "Comparisons are published once per job")


async def shadowed_holder(infra):
    """Production version 3, shadow versions 2 and 1: the newest shadow (2) is loaded."""
    prod = infra.publish_version(3)
    shadows = [infra.publish_version(n) for n in (2, 1)]
    for v in shadows:
        v.stage = events.MODEL_VERSION_STAGE_SHADOW
    infra.registry.versions = [prod, *shadows]
    holder = infra.holder()
    await holder.reload()
    return holder


@pytest.mark.spec(*SHADOW_RUNS)
@pytest.mark.spec(*SHADOW_EVENT)
async def test_shadow_loaded(infra):
    holder = await shadowed_holder(infra)
    assert holder.current.version == 3 and holder.shadow.version == 2
    sub = await infra.consumer()
    job_id = await infra.submit(jpeg())

    p = processor(infra, holder, shadow=lambda: holder.shadow)
    assert await p.handle(await fetch_one(sub)) == "succeeded"

    job, _ = await infra.job(job_id)
    assert job.prediction.model_version_id == "version-3"
    completed = ipb.InferenceCompleted.FromString(
        (await infra.messages(f"inference.completed.{infra.model}"))[0].data
    )
    assert completed.prediction.model_version_id == "version-3"
    (msg,) = await infra.messages(f"inference.shadow.{infra.model}")
    assert msg.headers["Nats-Msg-Id"] == f"shadow-{job_id}"
    event = ipb.ShadowCompared.FromString(msg.data)
    assert event.job_id == job_id and event.model_name == infra.model
    assert (event.production_version_id, event.shadow_version_id) == ("version-3", "version-2")
    assert event.production_label == job.prediction.label and event.shadow_label
    assert event.agree == (event.production_label == event.shadow_label)
    assert event.production_ms > 0 and event.shadow_ms > 0


@pytest.mark.spec(*SHADOW_RUNS)
async def test_shadow_fails(infra):
    holder = await loaded_holder(infra, number=3)

    def broken(x):
        raise RuntimeError("shadow exploded")

    bad = LoadedModel(version_id="version-9", version=9, framework="pytorch", predict=broken)
    sub = await infra.consumer()
    job_id = await infra.submit(jpeg())

    outcome = await processor(infra, holder, shadow=lambda: bad).handle(await fetch_one(sub))

    assert outcome == "succeeded"
    job, _ = await infra.job(job_id)
    assert job.status == ipb.JOB_STATUS_SUCCEEDED and job.prediction.model_version == 3
    assert await infra.count(f"inference.shadow.{infra.model}") == 0


@pytest.mark.spec(*SHADOW_RUNS)
async def test_shadow_with_bad_checksum_is_skipped(infra):
    prod = infra.publish_version(3)
    shadow = infra.publish_version(4, corrupt_checksum=True)
    shadow.stage = events.MODEL_VERSION_STAGE_SHADOW
    infra.registry.versions = [shadow, prod]
    holder = infra.holder()
    await holder.reload()  # must not raise: production is fine
    assert holder.current.version == 3 and holder.shadow is None


@pytest.mark.spec(*SHADOW_RUNS)
async def test_worker_follows_shadow_changes(infra):
    holder = infra.holder()
    entered = events.ModelVersionStageChanged(
        model_name=infra.model,
        from_stage=events.MODEL_VERSION_STAGE_VALIDATED,
        to_stage=events.MODEL_VERSION_STAGE_SHADOW,
    )
    assert holder.follows(entered)
    other = events.ModelVersionStageChanged(
        model_name=infra.model,
        from_stage=events.MODEL_VERSION_STAGE_REGISTERED,
        to_stage=events.MODEL_VERSION_STAGE_VALIDATED,
    )
    assert not holder.follows(other)
