import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ModelVersionStage(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    MODEL_VERSION_STAGE_UNSPECIFIED: _ClassVar[ModelVersionStage]
    MODEL_VERSION_STAGE_REGISTERED: _ClassVar[ModelVersionStage]
    MODEL_VERSION_STAGE_VALIDATED: _ClassVar[ModelVersionStage]
    MODEL_VERSION_STAGE_SHADOW: _ClassVar[ModelVersionStage]
    MODEL_VERSION_STAGE_PRODUCTION: _ClassVar[ModelVersionStage]
    MODEL_VERSION_STAGE_RETIRED: _ClassVar[ModelVersionStage]
    MODEL_VERSION_STAGE_REJECTED: _ClassVar[ModelVersionStage]
    MODEL_VERSION_STAGE_ROLLED_BACK: _ClassVar[ModelVersionStage]
MODEL_VERSION_STAGE_UNSPECIFIED: ModelVersionStage
MODEL_VERSION_STAGE_REGISTERED: ModelVersionStage
MODEL_VERSION_STAGE_VALIDATED: ModelVersionStage
MODEL_VERSION_STAGE_SHADOW: ModelVersionStage
MODEL_VERSION_STAGE_PRODUCTION: ModelVersionStage
MODEL_VERSION_STAGE_RETIRED: ModelVersionStage
MODEL_VERSION_STAGE_REJECTED: ModelVersionStage
MODEL_VERSION_STAGE_ROLLED_BACK: ModelVersionStage

class ModelVersionRegistered(_message.Message):
    __slots__ = ("event_id", "model_id", "model_name", "version_id", "version", "artifact_uri", "artifact_sha256", "framework", "metrics", "occurred_at")
    class MetricsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: float
        def __init__(self, key: _Optional[str] = ..., value: _Optional[float] = ...) -> None: ...
    EVENT_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    VERSION_ID_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    ARTIFACT_URI_FIELD_NUMBER: _ClassVar[int]
    ARTIFACT_SHA256_FIELD_NUMBER: _ClassVar[int]
    FRAMEWORK_FIELD_NUMBER: _ClassVar[int]
    METRICS_FIELD_NUMBER: _ClassVar[int]
    OCCURRED_AT_FIELD_NUMBER: _ClassVar[int]
    event_id: str
    model_id: str
    model_name: str
    version_id: str
    version: int
    artifact_uri: str
    artifact_sha256: str
    framework: str
    metrics: _containers.ScalarMap[str, float]
    occurred_at: _timestamp_pb2.Timestamp
    def __init__(self, event_id: _Optional[str] = ..., model_id: _Optional[str] = ..., model_name: _Optional[str] = ..., version_id: _Optional[str] = ..., version: _Optional[int] = ..., artifact_uri: _Optional[str] = ..., artifact_sha256: _Optional[str] = ..., framework: _Optional[str] = ..., metrics: _Optional[_Mapping[str, float]] = ..., occurred_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ModelVersionStageChanged(_message.Message):
    __slots__ = ("event_id", "model_id", "model_name", "version_id", "version", "from_stage", "to_stage", "reason", "revision", "occurred_at")
    EVENT_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    VERSION_ID_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    FROM_STAGE_FIELD_NUMBER: _ClassVar[int]
    TO_STAGE_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    REVISION_FIELD_NUMBER: _ClassVar[int]
    OCCURRED_AT_FIELD_NUMBER: _ClassVar[int]
    event_id: str
    model_id: str
    model_name: str
    version_id: str
    version: int
    from_stage: ModelVersionStage
    to_stage: ModelVersionStage
    reason: str
    revision: int
    occurred_at: _timestamp_pb2.Timestamp
    def __init__(self, event_id: _Optional[str] = ..., model_id: _Optional[str] = ..., model_name: _Optional[str] = ..., version_id: _Optional[str] = ..., version: _Optional[int] = ..., from_stage: _Optional[_Union[ModelVersionStage, str]] = ..., to_stage: _Optional[_Union[ModelVersionStage, str]] = ..., reason: _Optional[str] = ..., revision: _Optional[int] = ..., occurred_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...
