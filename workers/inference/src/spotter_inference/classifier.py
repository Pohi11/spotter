"""The worker's synchronous gRPC classifier (change phase-11-failure-lab): the same model
and the same classify() as the bus path, called directly by the gateway's ClassifySync."""

import asyncio
import io
from collections.abc import Callable

import grpc
from PIL import Image, UnidentifiedImageError
from spotter.v1 import classifier_pb2 as cpb
from spotter.v1 import classifier_pb2_grpc as crpc

from spotter_inference.models import LoadedModel
from spotter_inference.processor import classify


class Classifier(crpc.ClassifierServiceServicer):
    def __init__(self, model: Callable[[], LoadedModel | None]) -> None:
        self._model = model

    async def Classify(self, request, context):  # noqa: N802 - gRPC method name
        loaded = self._model()
        if loaded is None:
            await context.abort(grpc.StatusCode.UNAVAILABLE, "no production model version loaded")
        try:
            with Image.open(io.BytesIO(request.image)) as img:
                img.load()
                image = img.copy()
        except (UnidentifiedImageError, OSError) as e:
            await context.abort(
                grpc.StatusCode.INVALID_ARGUMENT, f"image could not be decoded: {e}"
            )
        prediction, stats, ms = await asyncio.to_thread(classify, loaded, image)
        return cpb.ClassifyResponse(prediction=prediction, input_stats=stats, inference_ms=ms)


async def serve_classifier(port: int, model: Callable[[], LoadedModel | None]) -> grpc.aio.Server:
    """Start the gRPC classifier on port (0 picks one; read it from the returned server)."""
    server = grpc.aio.server()
    crpc.add_ClassifierServiceServicer_to_server(Classifier(model), server)
    server.bound_port = server.add_insecure_port(f"[::]:{port}")
    await server.start()
    return server
