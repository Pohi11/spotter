package natstrace_test

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/Pohi11/spotter/internal/natstrace"
)

func TestInjectThenExtractKeepsTheTrace(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	tp := sdktrace.NewTracerProvider()
	ctx, span := tp.Tracer("test").Start(context.Background(), "publish")
	defer span.End()

	msg := &nats.Msg{Subject: "inference.requested.x"}
	natstrace.Inject(ctx, msg)
	if msg.Header.Get("traceparent") == "" {
		t.Fatal("no traceparent header injected")
	}
	got := trace.SpanContextFromContext(natstrace.Extract(context.Background(), msg.Header))
	if got.TraceID() != span.SpanContext().TraceID() || got.SpanID() != span.SpanContext().SpanID() {
		t.Fatalf("extracted %v, want %v", got, span.SpanContext())
	}
}

func TestExtractWithoutHeadersIsHarmless(t *testing.T) {
	ctx := natstrace.Extract(context.Background(), nil)
	if trace.SpanContextFromContext(ctx).IsValid() {
		t.Fatal("found a trace in a message without headers")
	}
}
