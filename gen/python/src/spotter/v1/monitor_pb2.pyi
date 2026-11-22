import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class FeatureDrift(_message.Message):
    __slots__ = ("feature", "psi", "ks_statistic", "ks_p_value", "reference_mean", "window_mean")
    FEATURE_FIELD_NUMBER: _ClassVar[int]
    PSI_FIELD_NUMBER: _ClassVar[int]
    KS_STATISTIC_FIELD_NUMBER: _ClassVar[int]
    KS_P_VALUE_FIELD_NUMBER: _ClassVar[int]
    REFERENCE_MEAN_FIELD_NUMBER: _ClassVar[int]
    WINDOW_MEAN_FIELD_NUMBER: _ClassVar[int]
    feature: str
    psi: float
    ks_statistic: float
    ks_p_value: float
    reference_mean: float
    window_mean: float
    def __init__(self, feature: _Optional[str] = ..., psi: _Optional[float] = ..., ks_statistic: _Optional[float] = ..., ks_p_value: _Optional[float] = ..., reference_mean: _Optional[float] = ..., window_mean: _Optional[float] = ...) -> None: ...

class DriftDetected(_message.Message):
    __slots__ = ("model_name", "version_id", "version", "features", "window_size", "detected_at")
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    VERSION_ID_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    FEATURES_FIELD_NUMBER: _ClassVar[int]
    WINDOW_SIZE_FIELD_NUMBER: _ClassVar[int]
    DETECTED_AT_FIELD_NUMBER: _ClassVar[int]
    model_name: str
    version_id: str
    version: int
    features: _containers.RepeatedCompositeFieldContainer[FeatureDrift]
    window_size: int
    detected_at: _timestamp_pb2.Timestamp
    def __init__(self, model_name: _Optional[str] = ..., version_id: _Optional[str] = ..., version: _Optional[int] = ..., features: _Optional[_Iterable[_Union[FeatureDrift, _Mapping]]] = ..., window_size: _Optional[int] = ..., detected_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class DriftResolved(_message.Message):
    __slots__ = ("model_name", "version_id", "version", "features", "resolved_at")
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    VERSION_ID_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    FEATURES_FIELD_NUMBER: _ClassVar[int]
    RESOLVED_AT_FIELD_NUMBER: _ClassVar[int]
    model_name: str
    version_id: str
    version: int
    features: _containers.RepeatedCompositeFieldContainer[FeatureDrift]
    resolved_at: _timestamp_pb2.Timestamp
    def __init__(self, model_name: _Optional[str] = ..., version_id: _Optional[str] = ..., version: _Optional[int] = ..., features: _Optional[_Iterable[_Union[FeatureDrift, _Mapping]]] = ..., resolved_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...
