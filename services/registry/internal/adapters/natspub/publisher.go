// Package natspub publishes registry outbox records to JetStream.
package natspub

import (
	"context"
	"strings"
	"unicode"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/Pohi11/spotter/internal/natsjs"
	"github.com/Pohi11/spotter/services/registry/internal/ports"
)

// Publisher implements ports.EventPublisher.
type Publisher struct{ JS jetstream.JetStream }

// Publish sends rec to registry.events.<event> with the record id as the message id, so
// the stream drops a re-publish within its duplicate window, and waits for the ack.
func (p Publisher) Publish(ctx context.Context, rec ports.OutboxRecord) error {
	msg := &nats.Msg{Subject: Subject(rec.Type), Data: rec.Payload, Header: nats.Header{}}
	msg.Header.Set(jetstream.MsgIDHeader, rec.ID.String())
	msg.Header.Set(natsjs.EventTypeHeader, rec.Type)
	if rec.TraceParent != "" {
		// The trace of the request that wrote the record, not of this relay pass.
		msg.Header.Set("traceparent", rec.TraceParent)
	}
	_, err := p.JS.PublishMsg(ctx, msg)
	return err
}

// Subject maps "spotter.v1.ModelVersionStageChanged" to
// "registry.events.model_version_stage_changed".
func Subject(eventType string) string {
	name := eventType[strings.LastIndex(eventType, ".")+1:]
	var b strings.Builder
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return "registry.events." + b.String()
}
