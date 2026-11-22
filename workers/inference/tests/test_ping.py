import asyncio
import os
from datetime import UTC, datetime

import nats
import pytest
from spotter.v1 import ping_pb2
from spotter_inference.__main__ import serve
from spotter_inference.ping import RESPONDER, SUBJECT, handle_ping


def test_reply_echoes_the_request_id():
    request = ping_pb2.PingRequest(id="abc123", sender="test")

    reply = ping_pb2.PingReply()
    reply.ParseFromString(handle_ping(request.SerializeToString()))

    assert reply.id == "abc123"
    assert reply.responder == RESPONDER
    assert reply.runtime.startswith("python 3.")


def test_reply_records_when_it_was_received():
    fixed = datetime(2026, 11, 22, 12, 0, tzinfo=UTC)

    reply = ping_pb2.PingReply()
    reply.ParseFromString(handle_ping(ping_pb2.PingRequest(id="x").SerializeToString(), now=fixed))

    assert reply.received_at.ToDatetime(tzinfo=UTC) == fixed


@pytest.mark.integration
async def test_worker_answers_ping_over_real_nats():
    url = os.environ.get("NATS_URL")
    if not url:
        pytest.skip("NATS_URL not set")

    stop = asyncio.Event()
    worker = asyncio.create_task(serve(url, stop))
    client = await nats.connect(url)
    try:
        request = ping_pb2.PingRequest(id="it-1", sender="pytest")
        # Retry briefly: the worker subscribes asynchronously after connecting.
        for _ in range(20):
            try:
                msg = await client.request(SUBJECT, request.SerializeToString(), timeout=0.5)
                break
            except nats.errors.NoRespondersError:
                await asyncio.sleep(0.05)
        else:
            pytest.fail("worker never subscribed")

        reply = ping_pb2.PingReply()
        reply.ParseFromString(msg.data)
        assert reply.id == "it-1"
        assert reply.responder == RESPONDER
    finally:
        await client.close()
        stop.set()
        await asyncio.wait_for(worker, timeout=5)
