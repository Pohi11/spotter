import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class PingRequest(_message.Message):
    __slots__ = ("id", "sender", "sent_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    SENDER_FIELD_NUMBER: _ClassVar[int]
    SENT_AT_FIELD_NUMBER: _ClassVar[int]
    id: str
    sender: str
    sent_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., sender: _Optional[str] = ..., sent_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class PingReply(_message.Message):
    __slots__ = ("id", "responder", "runtime", "received_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    RESPONDER_FIELD_NUMBER: _ClassVar[int]
    RUNTIME_FIELD_NUMBER: _ClassVar[int]
    RECEIVED_AT_FIELD_NUMBER: _ClassVar[int]
    id: str
    responder: str
    runtime: str
    received_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., responder: _Optional[str] = ..., runtime: _Optional[str] = ..., received_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...
