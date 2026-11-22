"""The worker's Prometheus metrics, served at /metrics on METRICS_PORT (decision record 0010)."""

from prometheus_client import Counter, Gauge, Histogram

JOBS = Counter(
    "spotter_inference_jobs_total",
    "Inference messages handled, by outcome (succeeded, failed, retried, duplicate).",
    ["outcome"],
)
MODEL_SECONDS = Histogram(
    "spotter_inference_model_seconds",
    "Time spent in the model for one image.",
    buckets=(0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0),
)
CONSUMER_PENDING = Gauge(
    "spotter_inference_consumer_pending",
    "Messages waiting for this worker's JetStream consumer.",
)
