from spotter.v1 import inference_pb2 as _inference_pb2
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ClassifyRequest(_message.Message):
    __slots__ = ("model_name", "image")
    MODEL_NAME_FIELD_NUMBER: _ClassVar[int]
    IMAGE_FIELD_NUMBER: _ClassVar[int]
    model_name: str
    image: bytes
    def __init__(self, model_name: _Optional[str] = ..., image: _Optional[bytes] = ...) -> None: ...

class ClassifyResponse(_message.Message):
    __slots__ = ("prediction", "input_stats", "inference_ms")
    PREDICTION_FIELD_NUMBER: _ClassVar[int]
    INPUT_STATS_FIELD_NUMBER: _ClassVar[int]
    INFERENCE_MS_FIELD_NUMBER: _ClassVar[int]
    prediction: _inference_pb2.Prediction
    input_stats: _inference_pb2.InputStats
    inference_ms: float
    def __init__(self, prediction: _Optional[_Union[_inference_pb2.Prediction, _Mapping]] = ..., input_stats: _Optional[_Union[_inference_pb2.InputStats, _Mapping]] = ..., inference_ms: _Optional[float] = ...) -> None: ...
