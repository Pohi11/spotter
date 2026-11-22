// Package telemetry sets up OpenTelemetry tracing and Prometheus metrics for a Go service
// (decision record 0010).
//
// Propagation (W3C TraceContext and Baggage) is always installed, so a service passes
// trace context on even when it exports nothing. Spans are exported over OTLP/gRPC only
// when OTEL_EXPORTER_OTLP_ENDPOINT is set.
package telemetry

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Telemetry holds what Setup created.
type Telemetry struct {
	// Metrics serves the Prometheus text format; mount it at /metrics.
	Metrics  http.Handler
	shutdown []func(context.Context) error
}

// Shutdown flushes and stops the exporters.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	var errs []error
	for _, f := range t.shutdown {
		errs = append(errs, f(ctx))
	}
	return errors.Join(errs...)
}

// TraceParent returns the W3C traceparent of the span active in ctx, or "" if none.
func TraceParent(ctx context.Context) string {
	c := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, c)
	return c["traceparent"]
}

// Setup installs the global propagator, tracer provider, and meter provider.
// service is used when OTEL_SERVICE_NAME is unset.
func Setup(ctx context.Context, service string) (*Telemetry, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	name := os.Getenv("OTEL_SERVICE_NAME")
	if name == "" {
		name = service
	}
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(semconv.ServiceName(name)))
	if err != nil {
		return nil, err
	}
	t := &Telemetry{}

	traceOpts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		exp, err := otlptracegrpc.New(ctx) // reads the standard OTEL_EXPORTER_OTLP_* variables
		if err != nil {
			return nil, err
		}
		traceOpts = append(traceOpts, sdktrace.WithBatcher(exp))
	}
	tp := sdktrace.NewTracerProvider(traceOpts...)
	otel.SetTracerProvider(tp)
	t.shutdown = append(t.shutdown, tp.Shutdown)

	registry := prometheus.NewRegistry()
	exporter, err := promexporter.New(promexporter.WithRegisterer(registry))
	if err != nil {
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithResource(res), sdkmetric.WithReader(exporter))
	otel.SetMeterProvider(mp)
	t.shutdown = append(t.shutdown, mp.Shutdown)
	t.Metrics = promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	return t, nil
}
