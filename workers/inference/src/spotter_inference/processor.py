"""Turns InferenceRequested messages into results under at-least-once delivery.

Decision record 0009 and design D3/D4 of the event-driven inference change:
- record a result only if the job is still PENDING (compare-and-set on the KV revision);
- publish the completion with the job id as message id (the stream drops duplicates);
- ack only after both; nak with a delay on transient failures; term poison messages and
  the last allowed delivery after recording FAILED and publishing InferenceFailed.

When a shadow version is loaded (decision record 0013), the image is classified again with
it after the job is acknowledged, and a ShadowCompared event is published. Nothing the
shadow model does can change the job's result.
"""

import asyncio
import io
import logging
import time
from collections.abc import Callable
from datetime import UTC, datetime

import numpy as np
from google.protobuf.timestamp_pb2 import Timestamp
from nats.js.errors import KeyNotFoundError, KeyWrongLastSequenceError
from opentelemetry import propagate, trace
from opentelemetry.trace import SpanKind
from PIL import Image, UnidentifiedImageError
from spotter.v1 import inference_pb2 as pb
from spotter_model import CLASSES, preprocess

from spotter_inference import metrics
from spotter_inference.models import LoadedModel, split_s3_uri

log = logging.getLogger("spotter_inference.processor")
tracer = trace.get_tracer("spotter_inference.processor")

FINAL = (pb.JOB_STATUS_SUCCEEDED, pb.JOB_STATUS_FAILED)


def _now() -> Timestamp:
    ts = Timestamp()
    ts.FromDatetime(datetime.now(UTC))
    return ts


def classify(model: LoadedModel, image: Image.Image) -> tuple[pb.Prediction, pb.InputStats, float]:
    """Run the model on one image; return the prediction, input statistics, and model time."""
    pixels = np.asarray(image.convert("RGB"), dtype=np.float32) / 255.0
    stats = pb.InputStats(
        channel_mean=pixels.mean(axis=(0, 1)).tolist(), channel_std=pixels.std(axis=(0, 1)).tolist()
    )
    x = preprocess(image).numpy()
    start = time.perf_counter()
    probs = model.predict(x)[0]  # PyTorch or ONNX Runtime: same call, same output
    ms = (time.perf_counter() - start) * 1000
    best = int(probs.argmax())
    prediction = pb.Prediction(
        label=CLASSES[best],
        confidence=float(probs[best]),
        scores=[
            pb.ClassScore(label=c, score=float(p)) for c, p in zip(CLASSES, probs, strict=True)
        ],
        model_version_id=model.version_id,
        model_version=model.version,
    )
    return prediction, stats, ms


class Processor:
    def __init__(
        self,
        *,
        js,
        kv,
        s3,
        model: Callable[[], LoadedModel | None],
        max_deliver: int,
        shadow: Callable[[], LoadedModel | None] = lambda: None,
        retry_delay: float = 5.0,
        before_ack: Callable[[], None] | None = None,
    ) -> None:
        self.js, self.kv, self.s3 = js, kv, s3
        self.model = model
        self.shadow = shadow
        self.max_deliver = max_deliver
        self.retry_delay = retry_delay
        self.before_ack = before_ack  # test hook: raise here to simulate a crash before ack

    async def handle(self, msg) -> str:
        """Process one message and return its outcome: succeeded, failed, retried, or duplicate.

        The span continues the trace in the message's headers (decision record 0010); the
        outcome is a span attribute and a metric label.
        """
        parent = propagate.extract(dict(msg.headers or {}))
        with tracer.start_as_current_span(
            "process inference.requested",
            context=parent,
            kind=SpanKind.CONSUMER,
            attributes={
                "messaging.system": "nats",
                "messaging.destination.name": msg.subject,
                "messaging.message.delivery_count": msg.metadata.num_delivered,
            },
        ) as span:
            outcome = await self._handle(msg, span)
            span.set_attribute("spotter.outcome", outcome)
            metrics.JOBS.labels(outcome=outcome).inc()
            return outcome

    async def _handle(self, msg, span) -> str:
        deliveries = msg.metadata.num_delivered
        model_name = msg.subject.removeprefix("inference.requested.")
        try:
            req = pb.InferenceRequested.FromString(msg.data)
            if not req.job_id:
                raise ValueError("missing job_id")
        except Exception as e:  # noqa: BLE001 - anything undecodable is poison
            await self._dead_letter("", model_name, f"undecodable request: {e}", deliveries)
            await msg.term()
            return "failed"
        span.set_attributes({"spotter.job_id": req.job_id, "spotter.model": req.model_name})

        try:
            entry = await self.kv.get(req.job_id)
        except KeyNotFoundError:
            await self._dead_letter(req.job_id, model_name, "unknown job", deliveries)
            await msg.term()
            return "failed"
        except Exception as e:  # noqa: BLE001
            return await self._retry_or_fail(msg, req, None, f"job store unavailable: {e}")

        job = pb.InferenceJob.FromString(entry.value)
        if job.status in FINAL:
            # A redelivery of a job that already finished: re-publish, ack, record nothing.
            await self._publish_outcome(job, deliveries)
            await self._ack(msg)
            return "duplicate"

        loaded = self.model()
        if loaded is None:
            return await self._retry_or_fail(msg, req, entry, "no production model version loaded")
        span.set_attribute("spotter.model_version", loaded.version)

        try:
            with tracer.start_as_current_span("download image") as s:
                bucket, key = split_s3_uri(req.image_uri)
                data = await asyncio.to_thread(
                    lambda: self.s3.get_object(Bucket=bucket, Key=key)["Body"].read()
                )
                s.set_attribute("spotter.image_bytes", len(data))
        except Exception as e:  # noqa: BLE001
            return await self._retry_or_fail(msg, req, entry, f"image unavailable: {e}")

        try:
            with tracer.start_as_current_span("classify"), Image.open(io.BytesIO(data)) as img:
                img.load()
                image = img.copy()
                prediction, stats, ms = await asyncio.to_thread(classify, loaded, image)
            metrics.MODEL_SECONDS.observe(ms / 1000)
        except (UnidentifiedImageError, OSError) as e:
            await self._fail(req, entry, f"image could not be decoded: {e}", deliveries)
            await msg.term()
            return "failed"

        job.status = pb.JOB_STATUS_SUCCEEDED
        job.prediction.CopyFrom(prediction)
        job.input_stats.CopyFrom(stats)
        job.inference_ms = ms
        job.completed_at.CopyFrom(_now())
        try:
            with tracer.start_as_current_span("record result"):
                await self.kv.update(req.job_id, job.SerializeToString(), last=entry.revision)
        except KeyWrongLastSequenceError:
            # Another delivery finished this job first; keep its result.
            current = pb.InferenceJob.FromString((await self.kv.get(req.job_id)).value)
            await self._publish_outcome(current, deliveries)
            await self._ack(msg)
            return "duplicate"

        await self._publish_outcome(job, deliveries)
        await self._ack(msg)
        await self._compare(job, loaded, image)
        return "succeeded"

    async def _compare(self, job: pb.InferenceJob, production: LoadedModel, image) -> None:
        """Classify with the shadow version and publish ShadowCompared; never raise.

        Runs only on the delivery that recorded the result, so a job is compared once; the
        message id `shadow-<job_id>` drops a second publish all the same.
        """
        shadow = self.shadow()
        if shadow is None or shadow.version_id == production.version_id:
            return
        try:
            with tracer.start_as_current_span(
                "shadow classify", attributes={"spotter.shadow_version": shadow.version}
            ):
                prediction, _, ms = await asyncio.to_thread(classify, shadow, image)
            agree = prediction.label == job.prediction.label
            event = pb.ShadowCompared(
                job_id=job.job_id,
                model_name=job.model_name,
                production_version_id=production.version_id,
                shadow_version_id=shadow.version_id,
                production_label=job.prediction.label,
                shadow_label=prediction.label,
                agree=agree,
                production_ms=job.inference_ms,
                shadow_ms=ms,
                compared_at=_now(),
            )
            subject = f"inference.shadow.{job.model_name}"
            with tracer.start_as_current_span(f"publish {subject}", kind=SpanKind.PRODUCER):
                headers = {
                    "Nats-Msg-Id": f"shadow-{job.job_id}",
                    "Spotter-Event-Type": "spotter.v1.ShadowCompared",
                }
                propagate.inject(headers)
                await self.js.publish(subject, event.SerializeToString(), headers=headers)
            metrics.SHADOW.labels(outcome="agree" if agree else "disagree").inc()
        except Exception as e:  # noqa: BLE001 - the shadow never affects the job
            log.warning("job %s: shadow comparison failed: %s", job.job_id, e)
            metrics.SHADOW.labels(outcome="error").inc()

    async def _ack(self, msg) -> None:
        if self.before_ack:
            self.before_ack()
        await msg.ack_sync()

    async def _retry_or_fail(self, msg, req, entry, reason: str) -> str:
        deliveries = msg.metadata.num_delivered
        if deliveries >= self.max_deliver:
            await self._fail(req, entry, f"{reason} (after {deliveries} deliveries)", deliveries)
            await msg.term()
            return "failed"
        log.warning("job %s: %s; retrying in %.1fs", req.job_id, reason, self.retry_delay)
        await msg.nak(delay=self.retry_delay)
        return "retried"

    async def _fail(self, req, entry, reason: str, deliveries: int) -> None:
        """Record FAILED (if still pending) and publish the dead letter."""
        if entry is not None:
            job = pb.InferenceJob.FromString(entry.value)
            if job.status not in FINAL:
                job.status = pb.JOB_STATUS_FAILED
                job.error = reason
                job.completed_at.CopyFrom(_now())
                try:
                    await self.kv.update(req.job_id, job.SerializeToString(), last=entry.revision)
                except KeyWrongLastSequenceError:
                    pass  # finished by another delivery
        await self._dead_letter(req.job_id, req.model_name, reason, deliveries)

    async def _publish_outcome(self, job: pb.InferenceJob, deliveries: int) -> None:
        if job.status == pb.JOB_STATUS_SUCCEEDED:
            event = pb.InferenceCompleted(
                job_id=job.job_id,
                model_name=job.model_name,
                prediction=job.prediction,
                input_stats=job.input_stats,
                inference_ms=job.inference_ms,
                completed_at=job.completed_at,
            )
            subject = f"inference.completed.{job.model_name}"
            with tracer.start_as_current_span(f"publish {subject}", kind=SpanKind.PRODUCER):
                headers = {
                    "Nats-Msg-Id": f"completed-{job.job_id}",
                    "Spotter-Event-Type": "spotter.v1.InferenceCompleted",
                }
                propagate.inject(headers)  # consumers of completions continue this trace
                await self.js.publish(subject, event.SerializeToString(), headers=headers)
        elif job.status == pb.JOB_STATUS_FAILED:
            await self._dead_letter(job.job_id, job.model_name, job.error, deliveries)

    async def _dead_letter(
        self, job_id: str, model_name: str, reason: str, deliveries: int
    ) -> None:
        event = pb.InferenceFailed(
            job_id=job_id,
            model_name=model_name,
            reason=reason,
            deliveries=deliveries,
            failed_at=_now(),
        )
        headers = {"Spotter-Event-Type": "spotter.v1.InferenceFailed"}
        if job_id:
            headers["Nats-Msg-Id"] = f"failed-{job_id}"
        propagate.inject(headers)
        log.error("job %s dead-lettered: %s", job_id or "?", reason)
        await self.js.publish(
            f"inference.failed.{model_name or 'unknown'}",
            event.SerializeToString(),
            headers=headers,
        )
