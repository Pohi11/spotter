"""Registers trained versions with the registry over gRPC, and applies the accuracy floor."""

import grpc
from spotter.v1 import registry_events_pb2 as events
from spotter.v1 import registry_pb2 as pb
from spotter.v1 import registry_pb2_grpc as rpc


class RegistryClient:
    def __init__(self, channel: grpc.Channel, timeout: float = 10.0) -> None:
        self._stub = rpc.RegistryServiceStub(channel)
        self._timeout = timeout

    def ensure_model(self, name: str, description: str) -> None:
        """Create the model, or do nothing if it already exists."""
        try:
            self._stub.RegisterModel(
                pb.RegisterModelRequest(name=name, description=description), timeout=self._timeout
            )
        except grpc.RpcError as e:
            if e.code() != grpc.StatusCode.ALREADY_EXISTS:
                raise

    def register_version(
        self, model_name: str, uri: str, sha256: str, framework: str, metrics: dict[str, float]
    ) -> pb.ModelVersion:
        resp = self._stub.RegisterVersion(
            pb.RegisterVersionRequest(
                model_name=model_name,
                artifact_uri=uri,
                artifact_sha256=sha256,
                framework=framework,
                metrics=metrics,
            ),
            timeout=self._timeout,
        )
        return resp.version

    def validate_if_accurate(self, version: pb.ModelVersion, accuracy: float, floor: float) -> bool:
        """Move a registered version to validated if accuracy meets the floor (design D7)."""
        if accuracy < floor:
            return False
        self._stub.TransitionVersion(
            pb.TransitionVersionRequest(
                version_id=version.id,
                to_stage=events.MODEL_VERSION_STAGE_VALIDATED,
                reason=f"holdout accuracy {accuracy:.4f} meets the floor {floor:.2f}",
                expected_revision=version.revision,
            ),
            timeout=self._timeout,
        )
        return True

    def to_shadow(self, version: pb.ModelVersion, reason: str) -> pb.ModelVersion:
        """Move a validated version to shadow; promotion still needs the gates and a person."""
        current = self._stub.GetVersion(
            pb.GetVersionRequest(version_id=version.id), timeout=self._timeout
        ).version
        return self._stub.TransitionVersion(
            pb.TransitionVersionRequest(
                version_id=version.id,
                to_stage=events.MODEL_VERSION_STAGE_SHADOW,
                reason=reason,
                expected_revision=current.revision,
            ),
            timeout=self._timeout,
        ).version
