"""Run the inference worker: `uv run spotter-inference` or `python -m spotter_inference`."""

import asyncio
import logging
import os
import signal

import nats
from nats.aio.msg import Msg

from spotter_inference.ping import RESPONDER, SUBJECT, handle_ping

log = logging.getLogger("spotter_inference")


async def serve(nats_url: str, stop: asyncio.Event) -> None:
    nc = await nats.connect(nats_url, name=RESPONDER)

    async def on_ping(msg: Msg) -> None:
        await msg.respond(handle_ping(msg.data))

    # A queue group: if several workers run, each ping is answered by exactly one of them.
    await nc.subscribe(SUBJECT, queue=RESPONDER, cb=on_ping)
    log.info("listening on %s at %s", SUBJECT, nats_url)

    await stop.wait()
    # Drain: stop taking new messages, finish the ones in flight, then close.
    await nc.drain()
    log.info("stopped")


async def main() -> None:
    stop = asyncio.Event()
    loop = asyncio.get_running_loop()
    for sig in (signal.SIGINT, signal.SIGTERM):
        try:
            loop.add_signal_handler(sig, stop.set)
        except NotImplementedError:
            # Windows event loops don't support signal handlers; Ctrl+C still raises
            # KeyboardInterrupt. In containers (Linux), SIGTERM takes this path.
            pass
    await serve(os.environ.get("NATS_URL", "nats://localhost:4222"), stop)


def run() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    run()
