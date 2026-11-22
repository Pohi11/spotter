import datetime

from google.api import annotations_pb2 as _annotations_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from spotter.v1 import registry_events_pb2 as _registry_events_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Gate(_message.Message):
    __slots__ = ("name", "passed", "detail")
    NAME_FIELD_NUMBER: _ClassVar[int]
    PASSED_FIELD_NUMBER: _ClassVar[int]
    DETAIL_FIELD_NUMBER: _ClassVar[int]
    name: str
    passed: bool
    detail: str
    def __init__(self, name: _Optional[str] = ..., passed: _Optional[bool] = ..., detail: _Optional[str] = ...) -> None: ...

class ApproveVersionRequest(_message.Message):
    __slots__ = ("version_id", "approver", "reason")
    VERSION_ID_FIELD_NUMBER: _ClassVar[int]
    APPROVER_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    version_id: str
    approver: str
    reason: str
    def __init__(self, version_id: _Optional[str] = ..., approver: _Optional[str] = ..., reason: _Optional[str] = ...) -> None: ...

class ApproveVersionResponse(_message.Message):
    __slots__ = ("version_id", "approver", "approved_at")
    VERSION_ID_FIELD_NUMBER: _ClassVar[int]
    APPROVER_FIELD_NUMBER: _ClassVar[int]
    APPROVED_AT_FIELD_NUMBER: _ClassVar[int]
    version_id: str
    approver: str
    approved_at: _timestamp_pb2.Timestamp
    def __init__(self, version_id: _Optional[str] = ..., approver: _Optional[str] = ..., approved_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GetPromotionGatesRequest(_message.Message):
    __slots__ = ("version_id",)
    VERSION_ID_FIELD_NUMBER: _ClassVar[int]
    version_id: str
    def __init__(self, version_id: _Optional[str] = ...) -> None: ...

class GetPromotionGatesResponse(_message.Message):
    __slots__ = ("gates", "all_passed")
    GATES_FIELD_NUMBER: _ClassVar[int]
    ALL_PASSED_FIELD_NUMBER: _ClassVar[int]
    gates: _containers.RepeatedCompositeFieldContainer[Gate]
    all_passed: bool
    def __init__(self, gates: _Optional[_Iterable[_Union[Gate, _Mapping]]] = ..., all_passed: _Optional[bool] = ...) -> None: ...

class PromoteVersionRequest(_message.Message):
    __slots__ = ("version_id", "reason")
    VERSION_ID_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    version_id: str
    reason: str
    def __init__(self, version_id: _Optional[str] = ..., reason: _Optional[str] = ...) -> None: ...

class PromoteVersionResponse(_message.Message):
    __slots__ = ("promoted", "retired")
    PROMOTED_FIELD_NUMBER: _ClassVar[int]
    RETIRED_FIELD_NUMBER: _ClassVar[int]
    promoted: ModelVersion
    retired: ModelVersion
    def __init__(self, promoted: _Optional[_Union[ModelVersion, _Mapping]] = ..., retired: _Optional[_Union[ModelVersion, _Mapping]] = ...) -> None: ...

class RollbackModelRequest(_message.Message):
    __slots__ = ("model_name", "reason")
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    model_name: str
    reason: str
    def __init__(self, model_name: _Optional[str] = ..., reason: _Optional[str] = ...) -> None: ...

class RollbackModelResponse(_message.Message):
    __slots__ = ("restored", "rolled_back")
    RESTORED_FIELD_NUMBER: _ClassVar[int]
    ROLLED_BACK_FIELD_NUMBER: _ClassVar[int]
    restored: ModelVersion
    rolled_back: ModelVersion
    def __init__(self, restored: _Optional[_Union[ModelVersion, _Mapping]] = ..., rolled_back: _Optional[_Union[ModelVersion, _Mapping]] = ...) -> None: ...

class Model(_message.Message):
    __slots__ = ("id", "name", "description", "created_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    description: str
    created_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., description: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ModelVersion(_message.Message):
    __slots__ = ("id", "model_id", "version", "artifact_uri", "artifact_sha256", "framework", "metrics", "stage", "revision", "created_at", "updated_at")
    class MetricsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: float
        def __init__(self, key: _Optional[str] = ..., value: _Optional[float] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_ID_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    ARTIFACT_URI_FIELD_NUMBER: _ClassVar[int]
    ARTIFACT_SHA256_FIELD_NUMBER: _ClassVar[int]
    FRAMEWORK_FIELD_NUMBER: _ClassVar[int]
    METRICS_FIELD_NUMBER: _ClassVar[int]
    STAGE_FIELD_NUMBER: _ClassVar[int]
    REVISION_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    UPDATED_AT_FIELD_NUMBER: _ClassVar[int]
    id: str
    model_id: str
    version: int
    artifact_uri: str
    artifact_sha256: str
    framework: str
    metrics: _containers.ScalarMap[str, float]
    stage: _registry_events_pb2.ModelVersionStage
    revision: int
    created_at: _timestamp_pb2.Timestamp
    updated_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., model_id: _Optional[str] = ..., version: _Optional[int] = ..., artifact_uri: _Optional[str] = ..., artifact_sha256: _Optional[str] = ..., framework: _Optional[str] = ..., metrics: _Optional[_Mapping[str, float]] = ..., stage: _Optional[_Union[_registry_events_pb2.ModelVersionStage, str]] = ..., revision: _Optional[int] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., updated_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class StageChange(_message.Message):
    __slots__ = ("from_stage", "to_stage", "reason", "revision", "changed_at")
    FROM_STAGE_FIELD_NUMBER: _ClassVar[int]
    TO_STAGE_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    REVISION_FIELD_NUMBER: _ClassVar[int]
    CHANGED_AT_FIELD_NUMBER: _ClassVar[int]
    from_stage: _registry_events_pb2.ModelVersionStage
    to_stage: _registry_events_pb2.ModelVersionStage
    reason: str
    revision: int
    changed_at: _timestamp_pb2.Timestamp
    def __init__(self, from_stage: _Optional[_Union[_registry_events_pb2.ModelVersionStage, str]] = ..., to_stage: _Optional[_Union[_registry_events_pb2.ModelVersionStage, str]] = ..., reason: _Optional[str] = ..., revision: _Optional[int] = ..., changed_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class RegisterModelRequest(_message.Message):
    __slots__ = ("name", "description")
    NAME_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    name: str
    description: str
    def __init__(self, name: _Optional[str] = ..., description: _Optional[str] = ...) -> None: ...

class RegisterModelResponse(_message.Message):
    __slots__ = ("model",)
    MODEL_FIELD_NUMBER: _ClassVar[int]
    model: Model
    def __init__(self, model: _Optional[_Union[Model, _Mapping]] = ...) -> None: ...

class GetModelRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class GetModelResponse(_message.Message):
    __slots__ = ("model",)
    MODEL_FIELD_NUMBER: _ClassVar[int]
    model: Model
    def __init__(self, model: _Optional[_Union[Model, _Mapping]] = ...) -> None: ...

class RegisterVersionRequest(_message.Message):
    __slots__ = ("model_name", "artifact_uri", "artifact_sha256", "framework", "metrics")
    class MetricsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: float
        def __init__(self, key: _Optional[str] = ..., value: _Optional[float] = ...) -> None: ...
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    ARTIFACT_URI_FIELD_NUMBER: _ClassVar[int]
    ARTIFACT_SHA256_FIELD_NUMBER: _ClassVar[int]
    FRAMEWORK_FIELD_NUMBER: _ClassVar[int]
    METRICS_FIELD_NUMBER: _ClassVar[int]
    model_name: str
    artifact_uri: str
    artifact_sha256: str
    framework: str
    metrics: _containers.ScalarMap[str, float]
    def __init__(self, model_name: _Optional[str] = ..., artifact_uri: _Optional[str] = ..., artifact_sha256: _Optional[str] = ..., framework: _Optional[str] = ..., metrics: _Optional[_Mapping[str, float]] = ...) -> None: ...

class RegisterVersionResponse(_message.Message):
    __slots__ = ("version",)
    VERSION_FIELD_NUMBER: _ClassVar[int]
    version: ModelVersion
    def __init__(self, version: _Optional[_Union[ModelVersion, _Mapping]] = ...) -> None: ...

class ListVersionsRequest(_message.Message):
    __slots__ = ("model_name",)
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    model_name: str
    def __init__(self, model_name: _Optional[str] = ...) -> None: ...

class ListVersionsResponse(_message.Message):
    __slots__ = ("versions",)
    VERSIONS_FIELD_NUMBER: _ClassVar[int]
    versions: _containers.RepeatedCompositeFieldContainer[ModelVersion]
    def __init__(self, versions: _Optional[_Iterable[_Union[ModelVersion, _Mapping]]] = ...) -> None: ...

class GetVersionRequest(_message.Message):
    __slots__ = ("version_id",)
    VERSION_ID_FIELD_NUMBER: _ClassVar[int]
    version_id: str
    def __init__(self, version_id: _Optional[str] = ...) -> None: ...

class GetVersionResponse(_message.Message):
    __slots__ = ("version",)
    VERSION_FIELD_NUMBER: _ClassVar[int]
    version: ModelVersion
    def __init__(self, version: _Optional[_Union[ModelVersion, _Mapping]] = ...) -> None: ...

class TransitionVersionRequest(_message.Message):
    __slots__ = ("version_id", "to_stage", "reason", "expected_revision")
    VERSION_ID_FIELD_NUMBER: _ClassVar[int]
    TO_STAGE_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    EXPECTED_REVISION_FIELD_NUMBER: _ClassVar[int]
    version_id: str
    to_stage: _registry_events_pb2.ModelVersionStage
    reason: str
    expected_revision: int
    def __init__(self, version_id: _Optional[str] = ..., to_stage: _Optional[_Union[_registry_events_pb2.ModelVersionStage, str]] = ..., reason: _Optional[str] = ..., expected_revision: _Optional[int] = ...) -> None: ...

class TransitionVersionResponse(_message.Message):
    __slots__ = ("version",)
    VERSION_FIELD_NUMBER: _ClassVar[int]
    version: ModelVersion
    def __init__(self, version: _Optional[_Union[ModelVersion, _Mapping]] = ...) -> None: ...

class GetVersionHistoryRequest(_message.Message):
    __slots__ = ("version_id",)
    VERSION_ID_FIELD_NUMBER: _ClassVar[int]
    version_id: str
    def __init__(self, version_id: _Optional[str] = ...) -> None: ...

class GetVersionHistoryResponse(_message.Message):
    __slots__ = ("changes",)
    CHANGES_FIELD_NUMBER: _ClassVar[int]
    changes: _containers.RepeatedCompositeFieldContainer[StageChange]
    def __init__(self, changes: _Optional[_Iterable[_Union[StageChange, _Mapping]]] = ...) -> None: ...
