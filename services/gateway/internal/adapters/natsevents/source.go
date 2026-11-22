// Package natsevents implements the gateway's event source with JetStream ordered
// consumers, one per stream (decision record 0016).
package natsevents

import (
	"context"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	// Links every spotter.v1 message into the global registry used to decode payloads.
	_ "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/natsjs"
	"github.com/Pohi11/spotter/services/gateway/internal/domain"
)

// Source reads the given streams. Ordered consumers are ephemeral and in memory: the
// source deletes them when its context ends, and the server removes any left behind by
// a crash after InactiveThreshold.
type Source struct {
	JS      jetstream.JetStream
	Streams []string
}

// New returns a source over the inference, registry, and monitor streams.
func New(js jetstream.JetStream) Source {
	return Source{JS: js, Streams: []string{natsjs.InferenceStream, natsjs.RegistryStream, natsjs.MonitorStream}}
}

// InactiveThreshold is how long the server keeps a consumer nobody reads.
const InactiveThreshold = 30 * time.Second

var jsonOpts = protojson.MarshalOptions{EmitUnpopulated: true}

type running struct {
	stream string
	cons   jetstream.Consumer
	cc     jetstream.ConsumeContext
}

func (s Source) Events(ctx context.Context, since time.Duration) (<-chan domain.Event, error) {
	start := time.Now().Add(-since)
	out := make(chan domain.Event, 256)
	var all []running
	stop := func() {
		for _, r := range all {
			r.cc.Stop()
			<-r.cc.Closed()
			if info := r.cons.CachedInfo(); info != nil {
				dctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				// Best effort: if this fails, InactiveThreshold removes it.
				_ = s.JS.DeleteConsumer(dctx, r.stream, info.Name)
				cancel()
			}
		}
	}
	for _, stream := range s.Streams {
		cons, err := s.JS.OrderedConsumer(ctx, stream, jetstream.OrderedConsumerConfig{
			DeliverPolicy: jetstream.DeliverByStartTimePolicy, OptStartTime: &start, InactiveThreshold: InactiveThreshold,
		})
		if err != nil {
			stop()
			return nil, err
		}
		cc, err := cons.Consume(func(msg jetstream.Msg) {
			select {
			case out <- Decode(stream, msg):
			case <-ctx.Done():
			}
		})
		if err != nil {
			stop()
			return nil, err
		}
		all = append(all, running{stream: stream, cons: cons, cc: cc})
	}
	go func() {
		<-ctx.Done()
		stop()
		close(out)
	}()
	return out, nil
}

// Decode turns a stream message into an event, decoding the payload by its
// Spotter-Event-Type header. An unknown type or an undecodable payload leaves Payload nil.
func Decode(stream string, msg jetstream.Msg) domain.Event {
	ev := domain.Event{Stream: stream, Subject: msg.Subject(), Type: msg.Headers().Get(natsjs.EventTypeHeader)}
	if meta, err := msg.Metadata(); err == nil {
		ev.Sequence = meta.Sequence.Stream
		ev.Time = meta.Timestamp.UTC()
	}
	if ev.Type == "" {
		return ev
	}
	mt, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(ev.Type))
	if err != nil {
		return ev
	}
	m := mt.New().Interface()
	if err := proto.Unmarshal(msg.Data(), m); err != nil {
		return ev
	}
	if b, err := jsonOpts.Marshal(m); err == nil {
		ev.Payload = b
	}
	return ev
}
