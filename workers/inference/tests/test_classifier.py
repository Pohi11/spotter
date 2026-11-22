import io

import grpc
import numpy as np
import pytest
from PIL import Image
from spotter.v1 import classifier_pb2 as cpb
from spotter.v1 import classifier_pb2_grpc as crpc
from spotter_inference.classifier import serve_classifier
from spotter_inference.models import LoadedModel

SYNC = ("inference-api", "Synchronous classification calls the worker directly")


def jpeg() -> bytes:
    buf = io.BytesIO()
    Image.fromarray(np.full((64, 64, 3), 90, dtype=np.uint8)).save(buf, format="JPEG")
    return buf.getvalue()


def forest_model() -> LoadedModel:
    def predict(x):
        probs = np.full((len(x), 10), 0.01, dtype=np.float32)
        probs[:, 1] = 0.91  # Forest
        return probs

    return LoadedModel(version_id="v3", version=3, framework="onnx", predict=predict)


async def call(model, image):
    server = await serve_classifier(0, lambda: model)
    try:
        async with grpc.aio.insecure_channel(f"localhost:{server.bound_port}") as ch:
            return await crpc.ClassifierServiceStub(ch).Classify(
                cpb.ClassifyRequest(model_name="eurosat-classifier", image=image), timeout=10
            )
    finally:
        await server.stop(None)


@pytest.mark.spec(*SYNC)
async def test_classifies_with_the_production_version():
    resp = await call(forest_model(), jpeg())
    assert resp.prediction.label == "Forest" and resp.prediction.model_version == 3
    assert len(resp.input_stats.channel_mean) == 3


@pytest.mark.spec(*SYNC)
async def test_no_model_loaded_is_unavailable():
    with pytest.raises(grpc.aio.AioRpcError) as e:
        await call(None, jpeg())
    assert e.value.code() == grpc.StatusCode.UNAVAILABLE


@pytest.mark.spec(*SYNC)
async def test_undecodable_image_is_invalid():
    with pytest.raises(grpc.aio.AioRpcError) as e:
        await call(forest_model(), b"not an image")
    assert e.value.code() == grpc.StatusCode.INVALID_ARGUMENT
