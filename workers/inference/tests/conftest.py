"""Real NATS JetStream and SeaweedFS (from Compose), a fake registry, and a tiny model."""

import asyncio
import hashlib
import io
import os
import uuid
from concurrent import futures
from dataclasses import dataclass, field

import boto3
import grpc
import nats
import pytest
import torch
from google.protobuf.timestamp_pb2 import Timestamp
from nats.errors import TimeoutError as NatsTimeout
from nats.js.api import AckPolicy, ConsumerConfig, StorageType, StreamConfig
from nats.js.errors import NotFoundError
from PIL import Image
from spotter.v1 import inference_pb2 as ipb
from spotter.v1 import registry_events_pb2 as events
from spotter.v1 import registry_pb2 as rpb
from spotter.v1 import registry_pb2_grpc as rpc
from spotter_inference.models import ModelHolder
from spotter_model import EuroSATNet


def artifact_bytes(net) -> bytes:
    buf = io.BytesIO()
    torch.save(net.state_dict(), buf)
    return buf.getvalue()


class FakeRegistry(rpc.RegistryServiceServicer):
    """Answers ListVersions with whatever versions a test sets."""

    def __init__(self) -> None:
        self.versions: list[rpb.ModelVersion] = []

    def ListVersions(self, request, context):
        return rpb.ListVersionsResponse(versions=self.versions)


@dataclass
class Infra:
    nc: object
    js: object
    kv: object
    s3: object
    bucket: str
    model: str
    registry: FakeRegistry
    registry_stub: rpc.RegistryServiceStub
    consumers: list[str] = field(default_factory=list)

    # --- model versions -------------------------------------------------------------
    def publish_version(self, number: int, *, corrupt_checksum: bool = False) -> rpb.ModelVersion:
        data = artifact_bytes(EuroSATNet())
        key = f"{self.model}/{hashlib.sha256(data).hexdigest()}/model.pt"
        self.s3.put_object(Bucket=self.bucket, Key=key, Body=data)
        sha = "0" * 64 if corrupt_checksum else hashlib.sha256(data).hexdigest()
        return rpb.ModelVersion(
            id=f"version-{number}",
            version=number,
            artifact_uri=f"s3://{self.bucket}/{key}",
            artifact_sha256=sha,
            framework="pytorch",
            stage=events.MODEL_VERSION_STAGE_PRODUCTION,
        )

    def holder(self) -> ModelHolder:
        return ModelHolder(self.registry_stub, self.s3, self.model)

    # --- jobs -----------------------------------------------------------------------
    async def submit(self, image: bytes) -> str:
        """Do what the gateway does: store the image, record PENDING, publish the request."""
        job_id = str(uuid.uuid4())
        key = f"{self.model}/{job_id}"
        self.s3.put_object(Bucket=self.bucket, Key=key, Body=image)
        ts = Timestamp()
        ts.GetCurrentTime()
        job = ipb.InferenceJob(
            job_id=job_id,
            model_name=self.model,
            status=ipb.JOB_STATUS_PENDING,
            image_uri=f"s3://{self.bucket}/{key}",
            submitted_at=ts,
        )
        await self.kv.create(job_id, job.SerializeToString())
        req = ipb.InferenceRequested(
            job_id=job_id, model_name=self.model, image_uri=job.image_uri, submitted_at=ts
        )
        await self.js.publish(
            f"inference.requested.{self.model}",
            req.SerializeToString(),
            headers={"Nats-Msg-Id": f"requested-{job_id}"},
        )
        return job_id

    async def job(self, job_id: str) -> tuple[ipb.InferenceJob, int]:
        entry = await self.kv.get(job_id)
        return ipb.InferenceJob.FromString(entry.value), entry.revision

    # --- consuming ------------------------------------------------------------------
    async def consumer(self, *, ack_wait: float = 1.0, max_deliver: int = 5):
        name = f"test-{uuid.uuid4().hex[:10]}"
        self.consumers.append(name)
        await self.js.add_consumer(
            "INFERENCE",
            ConsumerConfig(
                durable_name=name,
                filter_subject=f"inference.requested.{self.model}",
                ack_policy=AckPolicy.EXPLICIT,
                ack_wait=ack_wait,
                max_deliver=max_deliver,
            ),
        )
        return await self.js.pull_subscribe_bind(durable=name, stream="INFERENCE")

    async def count(self, subject: str) -> int:
        sub = await self.js.pull_subscribe(subject, stream="INFERENCE")
        try:
            return len(await sub.fetch(50, timeout=1))
        except NatsTimeout:
            return 0
        finally:
            await sub.unsubscribe()

    async def messages(self, subject: str) -> list:
        sub = await self.js.pull_subscribe(subject, stream="INFERENCE")
        try:
            return await sub.fetch(50, timeout=1)
        except NatsTimeout:
            return []
        finally:
            await sub.unsubscribe()


def jpeg(color=(40, 120, 60)) -> bytes:
    buf = io.BytesIO()
    Image.new("RGB", (64, 64), color).save(buf, format="JPEG")
    return buf.getvalue()


async def _ensure_streams(js) -> None:
    """Create the streams and bucket the Go services would (internal/natsjs), if missing."""
    for name, subjects, age in (
        ("INFERENCE", ["inference.>"], 86400),
        ("REGISTRY", ["registry.events.>"], 604800),
    ):
        try:
            await js.stream_info(name)
        except NotFoundError:
            await js.add_stream(
                StreamConfig(
                    name=name,
                    subjects=subjects,
                    max_age=age,
                    duplicate_window=120,
                    storage=StorageType.FILE,
                )
            )
    try:
        await js.key_value("inference_jobs")
    except Exception:  # noqa: BLE001 - bucket missing
        await js.create_key_value(bucket="inference_jobs", ttl=86400, history=1)


@pytest.fixture
async def infra():
    if not (os.environ.get("NATS_URL") and os.environ.get("S3_ENDPOINT")):
        pytest.skip("NATS_URL and S3_ENDPOINT not set")
    nc = await nats.connect(os.environ["NATS_URL"])
    js = nc.jetstream()
    await _ensure_streams(js)
    s3 = boto3.client(
        "s3",
        endpoint_url=os.environ["S3_ENDPOINT"],
        aws_access_key_id=os.environ["S3_ACCESS_KEY"],
        aws_secret_access_key=os.environ["S3_SECRET_KEY"],
        region_name=os.environ.get("S3_REGION", "us-east-1"),
    )
    bucket = f"test-{uuid.uuid4().hex[:12]}"
    s3.create_bucket(Bucket=bucket)

    fake = FakeRegistry()
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=2))
    rpc.add_RegistryServiceServicer_to_server(fake, server)
    port = server.add_insecure_port("127.0.0.1:0")
    server.start()
    channel = grpc.insecure_channel(f"127.0.0.1:{port}")

    i = Infra(
        nc=nc,
        js=js,
        kv=await js.key_value("inference_jobs"),
        s3=s3,
        bucket=bucket,
        model=f"m-{uuid.uuid4().hex[:10]}",
        registry=fake,
        registry_stub=rpc.RegistryServiceStub(channel),
    )
    yield i

    for name in i.consumers:
        try:
            await js.delete_consumer("INFERENCE", name)
        except Exception:  # noqa: BLE001
            pass
    await asyncio.to_thread(_empty_bucket, s3, bucket)
    channel.close()
    server.stop(None)
    await nc.close()


def _empty_bucket(s3, bucket: str) -> None:
    for obj in s3.list_objects_v2(Bucket=bucket).get("Contents", []):
        s3.delete_object(Bucket=bucket, Key=obj["Key"])
    s3.delete_bucket(Bucket=bucket)
