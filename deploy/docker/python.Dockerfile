# syntax=docker/dockerfile:1
# The inference worker and the trainer from the uv workspace (decision record 0014).
FROM ghcr.io/astral-sh/uv:0.11.28 AS uv

FROM python:3.13-slim-bookworm
COPY --from=uv /uv /usr/local/bin/uv
ENV UV_COMPILE_BYTECODE=1 UV_LINK_MODE=copy UV_PYTHON_DOWNLOADS=never \
    UV_PROJECT_ENVIRONMENT=/opt/venv PATH=/opt/venv/bin:$PATH PYTHONUNBUFFERED=1
WORKDIR /app

# Dependencies first, so source edits don't reinstall PyTorch. The uv cache stays in a
# BuildKit cache mount instead of the image.
COPY pyproject.toml uv.lock ./
COPY gen/python/pyproject.toml gen/python/
COPY workers/model/pyproject.toml workers/model/
COPY workers/inference/pyproject.toml workers/inference/
COPY workers/trainer/pyproject.toml workers/trainer/
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --locked --no-dev --no-install-workspace --all-packages

COPY gen/python gen/python
COPY workers workers
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --locked --no-dev --all-packages \
 && useradd --uid 10001 --no-create-home spotter
USER 10001
CMD ["spotter-inference"]
