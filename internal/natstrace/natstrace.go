// Package natstrace carries W3C trace context in NATS message headers, which NATS has no
// built-in instrumentation for (decision record 0010).
package natstrace

import (
	"context"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
)

// Carrier adapts nats.Header to OpenTelemetry's propagation.TextMapCarrier.
type Carrier nats.Header

func (c Carrier) Get(key string) string { return nats.Header(c).Get(key) }
func (c Carrier) Set(key, value string) { nats.Header(c).Set(key, value) }
func (c Carrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// Inject writes the trace context in ctx into msg's headers.
func Inject(ctx context.Context, msg *nats.Msg) {
	if msg.Header == nil {
		msg.Header = nats.Header{}
	}
	otel.GetTextMapPropagator().Inject(ctx, Carrier(msg.Header))
}

// Extract returns ctx with the trace context found in headers, if any.
func Extract(ctx context.Context, headers nats.Header) context.Context {
	if headers == nil {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, Carrier(headers))
}
