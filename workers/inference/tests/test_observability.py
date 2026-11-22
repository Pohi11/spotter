"""observability scenarios for the worker: traces across NATS, clean spans, outcome metrics."""

import os

import pytest
from prometheus_client import REGISTRY

from .conftest import SPANS, jpeg
from .test_processing import fetch_one, loaded_holder, processor

OBS = "observability"
pytestmark = pytest.mark.integration

TRACE_ID = "6bf92f3577b34da6a3ce929d0e0e4736"
TRACEPARENT = f"00-{TRACE_ID}-00f067aa0ba902b7-01"


def spans_of(trace_id: str):
    return [s for s in SPANS.get_finished_spans() if format(s.context.trace_id, "032x") == trace_id]


@pytest.mark.spec(OBS, "Every request continues or starts a trace")
async def test_worker_continues_the_trace_and_passes_it_on(infra):
    holder = await loaded_holder(infra)
    sub = await infra.consumer()
    job_id = await infra.submit(jpeg(), headers={"traceparent": TRACEPARENT})

    await processor(infra, holder).handle(await fetch_one(sub))

    names = {s.name for s in spans_of(TRACE_ID)}
    assert "process inference.requested" in names, names
    assert f"publish inference.completed.{infra.model}" in names, names
    (completed,) = await infra.messages(f"inference.completed.{infra.model}")
    assert TRACE_ID in completed.headers.get("traceparent", ""), completed.headers
    assert job_id


@pytest.mark.spec(OBS, "Spans never contain image bytes or secrets")
async def test_spans_hold_no_image_bytes_or_secrets(infra):
    holder = await loaded_holder(infra)
    sub = await infra.consumer()
    trace_id = "7bf92f3577b34da6a3ce929d0e0e4736"
    image = jpeg((1, 2, 3))
    await infra.submit(image, headers={"traceparent": f"00-{trace_id}-00f067aa0ba902b7-01"})

    await processor(infra, holder).handle(await fetch_one(sub))

    spans = spans_of(trace_id)
    assert spans
    secret = os.environ["S3_SECRET_KEY"]
    for span in spans:
        for key, value in (span.attributes or {}).items():
            text = repr(value)
            assert secret not in text, f"{span.name}.{key} leaks the S3 secret"
            assert repr(image)[2:40] not in text, f"{span.name}.{key} contains image bytes"


def outcome(name: str) -> float:
    return REGISTRY.get_sample_value("spotter_inference_jobs_total", {"outcome": name}) or 0.0


@pytest.mark.spec(OBS, "Services expose Prometheus metrics")
async def test_worker_outcome_metrics(infra):
    holder = await loaded_holder(infra)
    sub = await infra.consumer()
    before = outcome("succeeded"), outcome("failed")

    await infra.submit(jpeg())
    await processor(infra, holder).handle(await fetch_one(sub))
    await infra.submit(b"not an image")
    await processor(infra, holder).handle(await fetch_one(sub))

    assert outcome("succeeded") == before[0] + 1
    assert outcome("failed") == before[1] + 1
    assert (REGISTRY.get_sample_value("spotter_inference_model_seconds_count") or 0) >= 1
