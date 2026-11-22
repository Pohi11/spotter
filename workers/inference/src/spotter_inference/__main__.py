"""Run the inference worker: `uv run spotter-inference`.

Configuration from environment variables: NATS_URL, REGISTRY_ADDR (localhost:9090),
MODEL_NAME (eurosat-classifier), S3_ENDPOINT, S3_ACCESS_KEY, S3_SECRET_KEY,
S3_REGION (us-east-1), ACK_WAIT (30), MAX_DELIVER (5), RETRY_DELAY (5),
METRICS_PORT (9464), MODEL_THREADS (1), OTEL_EXPORTER_OTLP_ENDPOINT (unset: no export),
GRPC_PORT (unset: no synchronous classifier), SHADOW (on; off ignores shadow versions).
"""

import asyncio
import contextlib
import logging
import os
import signal
from dataclasses import dataclass

import boto3
import grpc
import nats
import prometheus_client
from nats.aio.msg import Msg
from nats.errors import TimeoutError as NatsTimeout
from nats.js.api import AckPolicy, ConsumerConfig, DeliverPolicy
from spotter.v1 import registry_events_pb2 as events
from spotter.v1 import registry_pb2_grpc as rpc

from spotter_inference import metrics, telemetry
from spotter_inference.classifier import serve_classifier
from spotter_inference.models import ChecksumMismatch, ModelHolder, configure_threads
from spotter_inference.ping import RESPONDER, SUBJECT, handle_ping
from spotter_inference.processor import Processor

log = logging.getLogger("spotter_inference")

INFERENCE_STREAM = "INFERENCE"
STAGE_CHANGED_SUBJECT = "registry.events.model_version_stage_changed"


@dataclass(frozen=True)
class Config:
    nats_url: str
    registry_addr: str
    model_name: str
    s3_endpoint: str
    s3_access_key: str
    s3_secret_key: str
    s3_region: str
    ack_wait: float
    max_deliver: int
    retry_delay: float
    metrics_port: int
    model_threads: int
    grpc_port: int | None = None
    shadow: bool = True

    @property
    def consumer(self) -> str:
        return f"inference-worker-{self.model_name}"

    @classmethod
    def from_env(cls) -> "Config":
        e = os.environ.get
        return cls(
            nats_url=e("NATS_URL", "nats://localhost:4222"),
            registry_addr=e("REGISTRY_ADDR", "localhost:9090"),
            model_name=e("MODEL_NAME", "eurosat-classifier"),
            s3_endpoint=e("S3_ENDPOINT", "http://localhost:8333"),
            s3_access_key=e("S3_ACCESS_KEY", ""),
            s3_secret_key=e("S3_SECRET_KEY", ""),
            s3_region=e("S3_REGION", "us-east-1"),
            ack_wait=float(e("ACK_WAIT", "30")),
            max_deliver=int(e("MAX_DELIVER", "5")),
            retry_delay=float(e("RETRY_DELAY", "5")),
            metrics_port=int(e("METRICS_PORT", "9464")),
            model_threads=int(e("MODEL_THREADS", "1")),
            grpc_port=int(e("GRPC_PORT")) if e("GRPC_PORT") else None,
            shadow=e("SHADOW", "on").lower() not in ("off", "false", "0"),
        )


def consumer_config(cfg: Config) -> ConsumerConfig:
    """The worker's durable pull consumer, declared in code (design D1)."""
    return ConsumerConfig(
        durable_name=cfg.consumer,
        filter_subject=f"inference.requested.{cfg.model_name}",
        ack_policy=AckPolicy.EXPLICIT,
        ack_wait=cfg.ack_wait,
        max_deliver=cfg.max_deliver,
        deliver_policy=DeliverPolicy.ALL,
    )


async def start_ping_responder(nc) -> None:
    """Answer spotter.v1.ping (one reply per ping across all workers, via a queue group)."""

    async def on_ping(msg: Msg) -> None:
        await msg.respond(handle_ping(msg.data))

    await nc.subscribe(SUBJECT, queue=RESPONDER, cb=on_ping)


async def wait_for_jetstream(js, stop: asyncio.Event):
    """Wait until the jobs bucket and streams exist (the Go services create them)."""
    while not stop.is_set():
        try:
            kv = await js.key_value("inference_jobs")
            await js.stream_info(INFERENCE_STREAM)
            await js.stream_info("REGISTRY")
            return kv
        except Exception as e:  # noqa: BLE001 - not created yet
            log.info("waiting for JetStream streams and bucket: %s", e)
            await asyncio.sleep(1)
    return None


async def consume(psub, processor: Processor, stop: asyncio.Event, before_fetch=None) -> None:
    """Fetch and process one message at a time until stop is set.

    One at a time because it measured better: in a closed-loop load test with two workers
    sharing the consumer, fetching batches of 8 gave a p99 latency of 973 ms and fetching
    one gave 169 ms (benchmarks/2026-11-22-load-*.json). The worker classifies one image at
    a time anyway, so batching the fetch gained nothing.
    """
    while not stop.is_set():
        if before_fetch:
            await before_fetch()
        try:
            with contextlib.suppress(Exception):  # the gauge is best effort
                metrics.CONSUMER_PENDING.set((await psub.consumer_info()).num_pending)
            msgs = await psub.fetch(1, timeout=1)
        except NatsTimeout:
            continue
        for msg in msgs:
            try:
                await processor.handle(msg)
            except Exception:  # noqa: BLE001 - never let one job kill the worker
                log.exception("job processing crashed; it will be redelivered")


async def serve(cfg: Config, stop: asyncio.Event) -> None:
    configure_threads(cfg.model_threads)
    nc = await nats.connect(cfg.nats_url, name=RESPONDER)
    await start_ping_responder(nc)  # first, so the worker answers pings while it waits
    prometheus_client.start_http_server(cfg.metrics_port)

    js = nc.jetstream()
    kv = await wait_for_jetstream(js, stop)
    if kv is None:
        await nc.drain()
        return
    s3 = boto3.client(
        "s3",
        endpoint_url=cfg.s3_endpoint,
        aws_access_key_id=cfg.s3_access_key,
        aws_secret_access_key=cfg.s3_secret_key,
        region_name=cfg.s3_region,
    )
    channel = grpc.insecure_channel(cfg.registry_addr)
    holder = ModelHolder(rpc.RegistryServiceStub(channel), s3, cfg.model_name)

    async def reload() -> None:
        try:
            await holder.reload()
        except ChecksumMismatch:
            pass  # logged; keep serving nothing rather than an unverified model
        except Exception as e:  # noqa: BLE001 - registry or store down: try again later
            log.warning("model reload failed: %s", e)

    async def on_stage_changed(msg: Msg) -> None:
        if holder.follows(events.ModelVersionStageChanged.FromString(msg.data)):
            await reload()

    # New stage changes only; the initial state comes from the registry.
    await js.subscribe(
        STAGE_CHANGED_SUBJECT,
        cb=on_stage_changed,
        ordered_consumer=True,
        deliver_policy=DeliverPolicy.NEW,
    )
    await reload()

    await js.add_consumer(INFERENCE_STREAM, consumer_config(cfg))
    psub = await js.pull_subscribe_bind(durable=cfg.consumer, stream=INFERENCE_STREAM)
    processor = Processor(
        js=js,
        kv=kv,
        s3=s3,
        model=lambda: holder.current,
        shadow=(lambda: holder.shadow) if cfg.shadow else (lambda: None),
        max_deliver=cfg.max_deliver,
        retry_delay=cfg.retry_delay,
    )
    log.info("worker for %s listening (consumer %s)", cfg.model_name, cfg.consumer)
    classifier = None
    if cfg.grpc_port is not None:
        classifier = await serve_classifier(cfg.grpc_port, lambda: holder.current)
        log.info("synchronous classifier on port %d", classifier.bound_port)

    async def before_fetch() -> None:
        if holder.current is None:
            await reload()  # e.g. the registry was down at startup

    await consume(psub, processor, stop, before_fetch=before_fetch)
    if classifier is not None:
        await classifier.stop(grace=5)
    await nc.drain()
    channel.close()
    log.info("stopped")


async def main() -> None:
    stop = asyncio.Event()
    loop = asyncio.get_running_loop()
    for sig in (signal.SIGINT, signal.SIGTERM):
        with contextlib.suppress(NotImplementedError):  # Windows: Ctrl+C raises instead
            loop.add_signal_handler(sig, stop.set)
    await serve(Config.from_env(), stop)


def run() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    telemetry.setup()
    with contextlib.suppress(KeyboardInterrupt):
        asyncio.run(main())


if __name__ == "__main__":
    run()
