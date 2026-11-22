"""Loading the production model version: find it, download it, verify it, swap it in."""

import asyncio
import hashlib
import io
import logging
from dataclasses import dataclass

import torch
from spotter.v1 import registry_events_pb2 as events
from spotter.v1 import registry_pb2 as pb
from spotter.v1 import registry_pb2_grpc as rpc
from spotter_model import EuroSATNet

log = logging.getLogger("spotter_inference.models")


class ChecksumMismatch(Exception):
    """The artifact's bytes don't match the checksum registered for the version."""


@dataclass(frozen=True)
class LoadedModel:
    version_id: str
    version: int
    net: torch.nn.Module


def split_s3_uri(uri: str) -> tuple[str, str]:
    bucket, _, key = uri.removeprefix("s3://").partition("/")
    return bucket, key


def load_version(s3, version: pb.ModelVersion) -> LoadedModel:
    """Download a version's artifact, verify its SHA-256, and load the weights."""
    bucket, key = split_s3_uri(version.artifact_uri)
    data = s3.get_object(Bucket=bucket, Key=key)["Body"].read()
    actual = hashlib.sha256(data).hexdigest()
    if actual != version.artifact_sha256:
        raise ChecksumMismatch(
            f"version {version.id}: sha256 {actual} != registered {version.artifact_sha256}"
        )
    net = EuroSATNet()
    net.load_state_dict(torch.load(io.BytesIO(data), weights_only=True))
    net.eval()
    return LoadedModel(version_id=version.id, version=version.version, net=net)


class ModelHolder:
    """Holds the currently loaded production model and reloads it on demand."""

    def __init__(self, registry: rpc.RegistryServiceStub, s3, model_name: str) -> None:
        self._registry = registry
        self._s3 = s3
        self._model_name = model_name
        self._lock = asyncio.Lock()
        self.current: LoadedModel | None = None

    def _production_version(self) -> pb.ModelVersion | None:
        resp = self._registry.ListVersions(
            pb.ListVersionsRequest(model_name=self._model_name), timeout=10
        )
        for v in resp.versions:
            if v.stage == events.MODEL_VERSION_STAGE_PRODUCTION:
                return v
        return None

    async def reload(self) -> None:
        """Load the production version if it changed; unload if there is none."""
        async with self._lock:
            version = await asyncio.to_thread(self._production_version)
            if version is None:
                if self.current is not None:
                    log.info("no production version of %s; unloading", self._model_name)
                self.current = None
                return
            if self.current is not None and self.current.version_id == version.id:
                return
            try:
                loaded = await asyncio.to_thread(load_version, self._s3, version)
            except ChecksumMismatch:
                log.error(
                    "refusing to load %s version %d: checksum mismatch",
                    self._model_name,
                    version.version,
                )
                self.current = None
                raise
            self.current = loaded  # one assignment: jobs see the old model or the new one
            log.info(
                "loaded %s version %d (%s)", self._model_name, loaded.version, loaded.version_id
            )

    def follows(self, event: events.ModelVersionStageChanged) -> bool:
        """Whether a stage change means the production version may have changed."""
        prod = events.MODEL_VERSION_STAGE_PRODUCTION
        return event.model_name == self._model_name and prod in (event.from_stage, event.to_stage)
