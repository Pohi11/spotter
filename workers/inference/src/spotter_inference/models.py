"""Loading model versions: find them, download them, verify them, swap them in.

The worker holds the production version and, when one exists, the newest shadow version
(decision record 0013). Results come from production only.
"""

import asyncio
import hashlib
import io
import logging
from collections.abc import Callable
from dataclasses import dataclass

import numpy as np
import torch
from spotter.v1 import registry_events_pb2 as events
from spotter.v1 import registry_pb2 as pb
from spotter.v1 import registry_pb2_grpc as rpc
from spotter_model import EuroSATNet

log = logging.getLogger("spotter_inference.models")


class ChecksumMismatch(Exception):
    """The artifact's bytes don't match the checksum registered for the version."""


Predict = Callable[[np.ndarray], np.ndarray]  # normalized float32 [N,3,64,64] -> probs [N,10]


@dataclass(frozen=True)
class LoadedModel:
    version_id: str
    version: int
    framework: str
    predict: Predict  # the same call whichever runtime is behind it


def split_s3_uri(uri: str) -> tuple[str, str]:
    bucket, _, key = uri.removeprefix("s3://").partition("/")
    return bucket, key


# Threads per model for intra-op parallelism. Both runtimes default to one thread per core,
# and ONNX Runtime's threads busy-wait between runs: with a production and a shadow session
# on a 22-thread laptop the worker used 16 cores to serve 10.8 jobs/s and starved the
# gateway, NATS, and the object store beside it. Batch-1 inference on this small network
# gains little from more threads; the worker scales by replicas instead.
_threads = 1


def configure_threads(n: int) -> None:
    """Set the intra-op threads for models loaded from now on (and for PyTorch globally)."""
    global _threads
    if n < 1:
        raise ValueError(f"MODEL_THREADS must be at least 1, got {n}")
    _threads = n
    torch.set_num_threads(n)


def _pytorch(data: bytes) -> Predict:
    net = EuroSATNet()
    net.load_state_dict(torch.load(io.BytesIO(data), weights_only=True))
    net.eval()

    def predict(x: np.ndarray) -> np.ndarray:
        with torch.no_grad():
            return torch.softmax(net(torch.from_numpy(x)), dim=1).numpy()

    return predict


def _onnx(data: bytes) -> Predict:
    import onnxruntime as ort

    ort.disable_telemetry_events()  # no outbound telemetry from a disconnected deployment
    opts = ort.SessionOptions()
    opts.intra_op_num_threads = _threads
    opts.inter_op_num_threads = 1
    opts.add_session_config_entry("session.intra_op.allow_spinning", "0")
    session = ort.InferenceSession(data, sess_options=opts, providers=["CPUExecutionProvider"])
    name = session.get_inputs()[0].name

    def predict(x: np.ndarray) -> np.ndarray:
        logits = session.run(None, {name: x})[0]
        e = np.exp(logits - logits.max(axis=1, keepdims=True))
        return e / e.sum(axis=1, keepdims=True)

    return predict


RUNTIMES = {"pytorch": _pytorch, "onnx": _onnx}


def load_version(s3, version: pb.ModelVersion) -> LoadedModel:
    """Download a version's artifact, verify its SHA-256, and load it with its runtime."""
    bucket, key = split_s3_uri(version.artifact_uri)
    data = s3.get_object(Bucket=bucket, Key=key)["Body"].read()
    actual = hashlib.sha256(data).hexdigest()
    if actual != version.artifact_sha256:
        raise ChecksumMismatch(
            f"version {version.id}: sha256 {actual} != registered {version.artifact_sha256}"
        )
    if version.framework not in RUNTIMES:
        raise ValueError(f"version {version.id}: unsupported framework {version.framework!r}")
    return LoadedModel(
        version_id=version.id,
        version=version.version,
        framework=version.framework,
        predict=RUNTIMES[version.framework](data),
    )


class ModelHolder:
    """Holds the loaded production and shadow models and reloads them on demand."""

    def __init__(self, registry: rpc.RegistryServiceStub, s3, model_name: str) -> None:
        self._registry = registry
        self._s3 = s3
        self._model_name = model_name
        self._lock = asyncio.Lock()
        self.current: LoadedModel | None = None
        self.shadow: LoadedModel | None = None

    def _versions(self) -> tuple[pb.ModelVersion | None, pb.ModelVersion | None]:
        """The production version and the newest shadow version (either may be None)."""
        resp = self._registry.ListVersions(
            pb.ListVersionsRequest(model_name=self._model_name), timeout=10
        )
        prod = shadow = None
        for v in resp.versions:  # highest version number first
            if v.stage == events.MODEL_VERSION_STAGE_PRODUCTION and prod is None:
                prod = v
            elif v.stage == events.MODEL_VERSION_STAGE_SHADOW and shadow is None:
                shadow = v
        return prod, shadow

    async def reload(self) -> None:
        """Load the production and shadow versions if they changed; unload what is gone.

        A shadow version that fails to load is logged and skipped: it never affects
        production. A production checksum mismatch unloads production and raises.
        """
        async with self._lock:
            prod, shadow = await asyncio.to_thread(self._versions)
            await self._reload_shadow(shadow)
            if prod is None:
                if self.current is not None:
                    log.info("no production version of %s; unloading", self._model_name)
                self.current = None
                return
            if self.current is not None and self.current.version_id == prod.id:
                return
            try:
                loaded = await asyncio.to_thread(load_version, self._s3, prod)
            except ChecksumMismatch:
                log.error(
                    "refusing to load %s version %d: checksum mismatch",
                    self._model_name,
                    prod.version,
                )
                self.current = None
                raise
            self.current = loaded  # one assignment: jobs see the old model or the new one
            log.info(
                "loaded %s version %d (%s)", self._model_name, loaded.version, loaded.version_id
            )

    async def _reload_shadow(self, version: pb.ModelVersion | None) -> None:
        if version is None:
            if self.shadow is not None:
                log.info("no shadow version of %s; unloading shadow", self._model_name)
            self.shadow = None
            return
        if self.shadow is not None and self.shadow.version_id == version.id:
            return
        try:
            self.shadow = await asyncio.to_thread(load_version, self._s3, version)
        except Exception as e:  # noqa: BLE001 - a bad shadow must not stop production
            log.error("not shadowing %s version %d: %s", self._model_name, version.version, e)
            self.shadow = None
            return
        log.info("shadowing %s version %d (%s)", self._model_name, version.version, version.id)

    def follows(self, event: events.ModelVersionStageChanged) -> bool:
        """Whether a stage change may change the production or shadow version."""
        watched = (events.MODEL_VERSION_STAGE_PRODUCTION, events.MODEL_VERSION_STAGE_SHADOW)
        return event.model_name == self._model_name and (
            event.from_stage in watched or event.to_stage in watched
        )
