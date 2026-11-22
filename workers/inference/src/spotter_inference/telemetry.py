"""OpenTelemetry setup for the worker (decision record 0010).

OpenTelemetry's default propagators (W3C TraceContext and Baggage) are used, so trace
context is passed on even when nothing is exported. Spans are exported over OTLP/gRPC only
when OTEL_EXPORTER_OTLP_ENDPOINT is set.
"""

import os

from opentelemetry import trace
from opentelemetry.sdk.resources import SERVICE_NAME, Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor


def setup(default_service: str = "inference-worker") -> TracerProvider:
    service = os.environ.get("OTEL_SERVICE_NAME", default_service)
    provider = TracerProvider(resource=Resource.create({SERVICE_NAME: service}))
    if os.environ.get("OTEL_EXPORTER_OTLP_ENDPOINT"):
        from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter

        provider.add_span_processor(BatchSpanProcessor(OTLPSpanExporter()))
    trace.set_tracer_provider(provider)
    return provider
