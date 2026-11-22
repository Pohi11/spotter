"""Ping responder: answers spotter.v1.ping so other services can check the worker is reachable."""

import platform
from datetime import UTC, datetime

from google.protobuf.timestamp_pb2 import Timestamp
from spotter.v1 import ping_pb2

SUBJECT = "spotter.v1.ping"
RESPONDER = "inference-worker"


def handle_ping(data: bytes, now: datetime | None = None) -> bytes:
    """Decode a PingRequest and encode the matching PingReply.

    Pure function: no I/O, so it can be tested without NATS.
    """
    request = ping_pb2.PingRequest()
    request.ParseFromString(data)

    received_at = Timestamp()
    received_at.FromDatetime(now or datetime.now(UTC))

    reply = ping_pb2.PingReply(
        id=request.id,
        responder=RESPONDER,
        runtime=f"python {platform.python_version()}",
        received_at=received_at,
    )
    return reply.SerializeToString()
