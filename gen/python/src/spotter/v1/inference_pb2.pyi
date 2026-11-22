import datetime

from google.api import annotations_pb2 as _annotations_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class JobStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    JOB_STATUS_UNSPECIFIED: _ClassVar[JobStatus]
    JOB_STATUS_PENDING: _ClassVar[JobStatus]
    JOB_STATUS_SUCCEEDED: _ClassVar[JobStatus]
    JOB_STATUS_FAILED: _ClassVar[JobStatus]
JOB_STATUS_UNSPECIFIED: JobStatus
JOB_STATUS_PENDING: JobStatus
JOB_STATUS_SUCCEEDED: JobStatus
JOB_STATUS_FAILED: JobStatus

class ClassScore(_message.Message):
    __slots__ = ("label", "score")
    LABEL_FIELD_NUMBER: _ClassVar[int]
    SCORE_FIELD_NUMBER: _ClassVar[int]
    label: str
    score: float
    def __init__(self, label: _Optional[str] = ..., score: _Optional[float] = ...) -> None: ...

class Prediction(_message.Message):
    __slots__ = ("label", "confidence", "scores", "model_version_id", "model_version")
    LABEL_FIELD_NUMBER: _ClassVar[int]
    CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    SCORES_FIELD_NUMBER: _ClassVar[int]
    MODEL_VERSION_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_VERSION_FIELD_NUMBER: _ClassVar[int]
    label: str
    confidence: float
    scores: _containers.RepeatedCompositeFieldContainer[ClassScore]
    model_version_id: str
    model_version: int
    def __init__(self, label: _Optional[str] = ..., confidence: _Optional[float] = ..., scores: _Optional[_Iterable[_Union[ClassScore, _Mapping]]] = ..., model_version_id: _Optional[str] = ..., model_version: _Optional[int] = ...) -> None: ...

class InferenceJob(_message.Message):
    __slots__ = ("job_id", "model_name", "status", "prediction", "error", "image_uri", "submitted_at", "completed_at", "input_stats", "inference_ms")
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    PREDICTION_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    IMAGE_URI_FIELD_NUMBER: _ClassVar[int]
    SUBMITTED_AT_FIELD_NUMBER: _ClassVar[int]
    COMPLETED_AT_FIELD_NUMBER: _ClassVar[int]
    INPUT_STATS_FIELD_NUMBER: _ClassVar[int]
    INFERENCE_MS_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    model_name: str
    status: JobStatus
    prediction: Prediction
    error: str
    image_uri: str
    submitted_at: _timestamp_pb2.Timestamp
    completed_at: _timestamp_pb2.Timestamp
    input_stats: InputStats
    inference_ms: float
    def __init__(self, job_id: _Optional[str] = ..., model_name: _Optional[str] = ..., status: _Optional[_Union[JobStatus, str]] = ..., prediction: _Optional[_Union[Prediction, _Mapping]] = ..., error: _Optional[str] = ..., image_uri: _Optional[str] = ..., submitted_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., completed_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., input_stats: _Optional[_Union[InputStats, _Mapping]] = ..., inference_ms: _Optional[float] = ...) -> None: ...

class SubmitInferenceRequest(_message.Message):
    __slots__ = ("model_name", "image")
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    IMAGE_FIELD_NUMBER: _ClassVar[int]
    model_name: str
    image: bytes
    def __init__(self, model_name: _Optional[str] = ..., image: _Optional[bytes] = ...) -> None: ...

class SubmitInferenceResponse(_message.Message):
    __slots__ = ("job_id",)
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    def __init__(self, job_id: _Optional[str] = ...) -> None: ...

class GetInferenceRequest(_message.Message):
    __slots__ = ("job_id",)
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    def __init__(self, job_id: _Optional[str] = ...) -> None: ...

class GetInferenceResponse(_message.Message):
    __slots__ = ("job",)
    JOB_FIELD_NUMBER: _ClassVar[int]
    job: InferenceJob
    def __init__(self, job: _Optional[_Union[InferenceJob, _Mapping]] = ...) -> None: ...

class InferenceRequested(_message.Message):
    __slots__ = ("job_id", "model_name", "image_uri", "submitted_at")
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    IMAGE_URI_FIELD_NUMBER: _ClassVar[int]
    SUBMITTED_AT_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    model_name: str
    image_uri: str
    submitted_at: _timestamp_pb2.Timestamp
    def __init__(self, job_id: _Optional[str] = ..., model_name: _Optional[str] = ..., image_uri: _Optional[str] = ..., submitted_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class InputStats(_message.Message):
    __slots__ = ("channel_mean", "channel_std")
    CHANNEL_MEAN_FIELD_NUMBER: _ClassVar[int]
    CHANNEL_STD_FIELD_NUMBER: _ClassVar[int]
    channel_mean: _containers.RepeatedScalarFieldContainer[float]
    channel_std: _containers.RepeatedScalarFieldContainer[float]
    def __init__(self, channel_mean: _Optional[_Iterable[float]] = ..., channel_std: _Optional[_Iterable[float]] = ...) -> None: ...

class InferenceCompleted(_message.Message):
    __slots__ = ("job_id", "model_name", "prediction", "input_stats", "inference_ms", "completed_at")
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    PREDICTION_FIELD_NUMBER: _ClassVar[int]
    INPUT_STATS_FIELD_NUMBER: _ClassVar[int]
    INFERENCE_MS_FIELD_NUMBER: _ClassVar[int]
    COMPLETED_AT_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    model_name: str
    prediction: Prediction
    input_stats: InputStats
    inference_ms: float
    completed_at: _timestamp_pb2.Timestamp
    def __init__(self, job_id: _Optional[str] = ..., model_name: _Optional[str] = ..., prediction: _Optional[_Union[Prediction, _Mapping]] = ..., input_stats: _Optional[_Union[InputStats, _Mapping]] = ..., inference_ms: _Optional[float] = ..., completed_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ShadowCompared(_message.Message):
    __slots__ = ("job_id", "model_name", "production_version_id", "shadow_version_id", "production_label", "shadow_label", "agree", "production_ms", "shadow_ms", "compared_at")
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    PRODUCTION_VERSION_ID_FIELD_NUMBER: _ClassVar[int]
    SHADOW_VERSION_ID_FIELD_NUMBER: _ClassVar[int]
    PRODUCTION_LABEL_FIELD_NUMBER: _ClassVar[int]
    SHADOW_LABEL_FIELD_NUMBER: _ClassVar[int]
    AGREE_FIELD_NUMBER: _ClassVar[int]
    PRODUCTION_MS_FIELD_NUMBER: _ClassVar[int]
    SHADOW_MS_FIELD_NUMBER: _ClassVar[int]
    COMPARED_AT_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    model_name: str
    production_version_id: str
    shadow_version_id: str
    production_label: str
    shadow_label: str
    agree: bool
    production_ms: float
    shadow_ms: float
    compared_at: _timestamp_pb2.Timestamp
    def __init__(self, job_id: _Optional[str] = ..., model_name: _Optional[str] = ..., production_version_id: _Optional[str] = ..., shadow_version_id: _Optional[str] = ..., production_label: _Optional[str] = ..., shadow_label: _Optional[str] = ..., agree: _Optional[bool] = ..., production_ms: _Optional[float] = ..., shadow_ms: _Optional[float] = ..., compared_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class InferenceFailed(_message.Message):
    __slots__ = ("job_id", "model_name", "reason", "deliveries", "failed_at")
    JOB_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    DELIVERIES_FIELD_NUMBER: _ClassVar[int]
    FAILED_AT_FIELD_NUMBER: _ClassVar[int]
    job_id: str
    model_name: str
    reason: str
    deliveries: int
    failed_at: _timestamp_pb2.Timestamp
    def __init__(self, job_id: _Optional[str] = ..., model_name: _Optional[str] = ..., reason: _Optional[str] = ..., deliveries: _Optional[int] = ..., failed_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...
