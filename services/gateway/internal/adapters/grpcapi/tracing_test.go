package grpcapi_test

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/natsjs"
	"github.com/Pohi11/spotter/internal/natstrace"
	"github.com/Pohi11/spotter/internal/s3test"
	"github.com/Pohi11/spotter/internal/spectest"
)

const observability = "observability"

// recorder captures every span the tests' servers create.
var recorder = tracetest.NewSpanRecorder()

func TestMain(m *testing.M) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)))
	os.Exit(m.Run())
}

func spansOfTrace(id string) []sdktrace.ReadOnlySpan {
	var out []sdktrace.ReadOnlySpan
	for _, s := range recorder.Ended() {
		if s.SpanContext().TraceID().String() == id {
			out = append(out, s)
		}
	}
	return out
}

// endedSpans waits up to five seconds for a trace's server and producer spans to end and
// returns every ended span of the trace. The gRPC server ends its span after it has sent
// the response, so a client can return from the call before that span is recorded.
func endedSpans(t *testing.T, id string) (spans []sdktrace.ReadOnlySpan, server, producer bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		spans = spansOfTrace(id)
		server, producer = false, false
		for _, s := range spans {
			switch s.SpanKind() {
			case trace.SpanKindServer:
				server = true
			case trace.SpanKindProducer:
				producer = true
			}
		}
		if (server && producer) || time.Now().After(deadline) {
			return spans, server, producer
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestGatewayContinuesTheCallersTraceOntoNATS(t *testing.T) {
	spectest.Verifies(t, observability, "Every request continues or starts a trace")
	const callerTrace = "4bf92f3577b34da6a3ce929d0e0e4736"
	e := setup(t)
	callCtx := metadata.AppendToOutgoingContext(ctx, "traceparent", "00-"+callerTrace+"-00f067aa0ba902b7-01")
	if _, err := e.client.SubmitInference(callCtx, &spotterv1.SubmitInferenceRequest{ModelName: "eurosat-classifier", Image: []byte("jpeg")}); err != nil {
		t.Fatal(err)
	}

	if _, server, producer := endedSpans(t, callerTrace); !server || !producer {
		t.Fatalf("caller's trace has server span %v and producer span %v; want both", server, producer)
	}

	cons, err := e.js.OrderedConsumer(ctx, natsjs.InferenceStream, jetstream.OrderedConsumerConfig{FilterSubjects: []string{natsjs.InferenceRequestedSubject("eurosat-classifier")}})
	if err != nil {
		t.Fatal(err)
	}
	msg, err := cons.Next()
	if err != nil {
		t.Fatal(err)
	}
	if got := trace.SpanContextFromContext(natstrace.Extract(ctx, msg.Headers())).TraceID().String(); got != callerTrace {
		t.Fatalf("message traceparent %q is in trace %s, want %s", msg.Headers().Get("traceparent"), got, callerTrace)
	}
}

func TestSpansHoldNoImageBytesOrSecrets(t *testing.T) {
	spectest.Verifies(t, observability, "Spans never contain image bytes or secrets")
	const id = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	e := setup(t)
	image := bytes.Repeat([]byte("IMAGEBYTES"), 50)
	callCtx := metadata.AppendToOutgoingContext(ctx, "traceparent", "00-"+id+"-00f067aa0ba902b7-01")
	if _, err := e.client.SubmitInference(callCtx, &spotterv1.SubmitInferenceRequest{ModelName: "eurosat-classifier", Image: image}); err != nil {
		t.Fatal(err)
	}
	secret := s3test.Start(t).SecretKey
	spans, server, producer := endedSpans(t, id)
	if !server || !producer {
		t.Fatalf("server span %v, producer span %v; want both checked", server, producer)
	}
	for _, s := range spans {
		for _, kv := range s.Attributes() {
			if v := kv.Value.Emit(); strings.Contains(v, "IMAGEBYTES") || strings.Contains(v, secret) {
				t.Fatalf("span %q attribute %s leaks data: %q", s.Name(), kv.Key, v)
			}
		}
	}
}
