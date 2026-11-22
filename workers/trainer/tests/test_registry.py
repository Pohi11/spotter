"""The registry client against an in-process fake gRPC registry (real gRPC, fake storage)."""

from concurrent import futures

import grpc
import pytest
from spotter.v1 import registry_events_pb2 as events
from spotter.v1 import registry_pb2 as pb
from spotter.v1 import registry_pb2_grpc as rpc
from spotter_trainer.registry import RegistryClient

REGISTERED = ("model-publishing", "Trained versions are registered with holdout metrics")
FLOOR = ("model-publishing", "Versions that meet the accuracy floor are validated")
SHA = "6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b"


class FakeRegistry(rpc.RegistryServiceServicer):
    def __init__(self) -> None:
        self.models: set[str] = set()
        self.versions: dict[str, list[pb.ModelVersion]] = {}
        self.transitions: list[pb.TransitionVersionRequest] = []

    def RegisterModel(self, request, context):
        if request.name in self.models:
            context.abort(grpc.StatusCode.ALREADY_EXISTS, "already exists")
        self.models.add(request.name)
        return pb.RegisterModelResponse(model=pb.Model(name=request.name))

    def RegisterVersion(self, request, context):
        if request.model_name not in self.models:
            context.abort(grpc.StatusCode.NOT_FOUND, "no such model")
        vs = self.versions.setdefault(request.model_name, [])
        v = pb.ModelVersion(
            id=f"v{len(vs) + 1}",
            version=len(vs) + 1,
            artifact_uri=request.artifact_uri,
            artifact_sha256=request.artifact_sha256,
            framework=request.framework,
            metrics=request.metrics,
            stage=events.MODEL_VERSION_STAGE_REGISTERED,
            revision=1,
        )
        vs.append(v)
        return pb.RegisterVersionResponse(version=v)

    def TransitionVersion(self, request, context):
        self.transitions.append(request)
        for vs in self.versions.values():
            for v in vs:
                if v.id == request.version_id:
                    v.stage = request.to_stage
                    v.revision += 1
                    return pb.TransitionVersionResponse(version=v)
        return pb.TransitionVersionResponse()


@pytest.fixture
def registry():
    fake = FakeRegistry()
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=2))
    rpc.add_RegistryServiceServicer_to_server(fake, server)
    port = server.add_insecure_port("127.0.0.1:0")
    server.start()
    with grpc.insecure_channel(f"127.0.0.1:{port}") as channel:
        yield fake, RegistryClient(channel)
    server.stop(None)


METRICS = {"accuracy": 0.93, "macro_f1": 0.92, "recall_Forest": 0.97}


@pytest.mark.spec(*REGISTERED)
def test_first_version_of_a_new_model(registry):
    fake, client = registry
    client.ensure_model("eurosat-classifier", "test")
    v = client.register_version(
        "eurosat-classifier", "s3://models/x/model.pt", SHA, "pytorch", METRICS
    )
    assert "eurosat-classifier" in fake.models
    assert v.version == 1 and v.artifact_sha256 == SHA
    assert dict(v.metrics) == pytest.approx(METRICS)


@pytest.mark.spec(*REGISTERED)
def test_model_already_exists(registry):
    fake, client = registry
    client.ensure_model("eurosat-classifier", "test")
    client.register_version("eurosat-classifier", "s3://models/a/model.pt", SHA, "pytorch", METRICS)
    client.ensure_model("eurosat-classifier", "test")  # ALREADY_EXISTS is not an error
    v = client.register_version(
        "eurosat-classifier", "s3://models/b/model.pt", SHA, "pytorch", METRICS
    )
    assert v.version == 2 and len(fake.versions["eurosat-classifier"]) == 2


@pytest.mark.spec(*FLOOR)
def test_above_the_floor_is_validated(registry):
    fake, client = registry
    client.ensure_model("eurosat-classifier", "test")
    v = client.register_version("eurosat-classifier", "s3://m/k", SHA, "pytorch", METRICS)
    assert client.validate_if_accurate(v, accuracy=0.93, floor=0.85)
    (t,) = fake.transitions
    assert t.to_stage == events.MODEL_VERSION_STAGE_VALIDATED and t.expected_revision == 1
    assert "0.93" in t.reason and "0.85" in t.reason


@pytest.mark.spec(*FLOOR)
def test_below_the_floor_stays_registered(registry):
    fake, client = registry
    client.ensure_model("eurosat-classifier", "test")
    v = client.register_version("eurosat-classifier", "s3://m/k", SHA, "pytorch", METRICS)
    assert not client.validate_if_accurate(v, accuracy=0.80, floor=0.85)
    assert fake.transitions == []
