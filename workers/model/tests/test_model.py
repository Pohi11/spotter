import torch
from PIL import Image
from spotter_model import CLASSES, EuroSATNet, preprocess


def test_ten_classes_in_torchvision_order():
    assert len(CLASSES) == 10
    assert list(CLASSES) == sorted(CLASSES)


def test_output_shape():
    model = EuroSATNet().eval()
    with torch.no_grad():
        out = model(torch.zeros(4, 3, 64, 64))
    assert out.shape == (4, len(CLASSES))


def test_preprocess_any_size_and_mode():
    for img in (Image.new("RGB", (64, 64), (10, 20, 30)), Image.new("L", (120, 90), 128)):
        x = preprocess(img)
        assert x.shape == (1, 3, 64, 64) and x.dtype == torch.float32
