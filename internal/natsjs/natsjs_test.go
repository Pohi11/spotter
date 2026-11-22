package natsjs_test

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/Pohi11/spotter/internal/natsjs"
	"github.com/Pohi11/spotter/internal/natstest"
	"github.com/Pohi11/spotter/internal/spectest"
)

const capability = "event-streams"

var ctx = context.Background()

func TestFirstStartCreatesStreamsAndBucket(t *testing.T) {
	spectest.Verifies(t, capability, "Streams and buckets are defined in code")
	_, js := natstest.Start(t).Connect(t)
	if err := natsjs.Ensure(ctx, js); err != nil {
		t.Fatal(err)
	}
	for name, subject := range map[string]string{natsjs.InferenceStream: "inference.>", natsjs.RegistryStream: "registry.events.>", natsjs.MonitorStream: "monitor.>"} {
		s, err := js.Stream(ctx, name)
		if err != nil {
			t.Fatalf("stream %s: %v", name, err)
		}
		cfg := s.CachedInfo().Config
		if len(cfg.Subjects) != 1 || cfg.Subjects[0] != subject || cfg.Duplicates < natsjs.DuplicateWindow {
			t.Fatalf("stream %s config = %+v", name, cfg)
		}
	}
	if _, err := js.KeyValue(ctx, natsjs.JobsBucket); err != nil {
		t.Fatalf("bucket: %v", err)
	}
}

func TestRepeatedStartKeepsMessages(t *testing.T) {
	spectest.Verifies(t, capability, "Streams and buckets are defined in code")
	_, js := natstest.Start(t).Connect(t)
	if err := natsjs.Ensure(ctx, js); err != nil {
		t.Fatal(err)
	}
	if _, err := js.Publish(ctx, "inference.requested.eurosat-classifier", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := natsjs.Ensure(ctx, js); err != nil {
		t.Fatalf("second Ensure: %v", err)
	}
	s, _ := js.Stream(ctx, natsjs.InferenceStream)
	info, err := s.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.State.Msgs != 1 {
		t.Fatalf("stream has %d messages after a second Ensure, want 1", info.State.Msgs)
	}
}

func TestDuplicateMessageIDIsStoredOnce(t *testing.T) {
	spectest.Verifies(t, capability, "Streams and buckets are defined in code")
	_, js := natstest.Start(t).Connect(t)
	if err := natsjs.Ensure(ctx, js); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		msg := &nats.Msg{Subject: "inference.requested.eurosat-classifier", Data: []byte("job"), Header: nats.Header{}}
		msg.Header.Set(jetstream.MsgIDHeader, "job-123")
		if _, err := js.PublishMsg(ctx, msg); err != nil {
			t.Fatal(err)
		}
	}
	s, _ := js.Stream(ctx, natsjs.InferenceStream)
	info, _ := s.Info(ctx)
	if info.State.Msgs != 1 {
		t.Fatalf("stream has %d messages, want 1", info.State.Msgs)
	}
}
