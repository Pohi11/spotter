package natspub_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Pohi11/spotter/internal/natsjs"
	"github.com/Pohi11/spotter/internal/natstest"
	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/services/registry/internal/adapters/natspub"
	"github.com/Pohi11/spotter/services/registry/internal/ports"
)

func TestSubject(t *testing.T) {
	if got := natspub.Subject("spotter.v1.ModelVersionStageChanged"); got != "registry.events.model_version_stage_changed" {
		t.Fatalf("Subject = %q", got)
	}
}

func TestRepublishedRecordIsStoredOnce(t *testing.T) {
	spectest.Verifies(t, "event-streams", "The registry publishes its outbox")
	ctx := context.Background()
	_, js := natstest.Start(t).Connect(t)
	if err := natsjs.Ensure(ctx, js); err != nil {
		t.Fatal(err)
	}
	rec := ports.OutboxRecord{ID: uuid.New(), Type: "spotter.v1.ModelVersionStageChanged", AggregateID: uuid.New(), Payload: []byte("event"), OccurredAt: time.Now()}
	pub := natspub.Publisher{JS: js}
	for range 2 { // published, "crash" before marking, published again
		if err := pub.Publish(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}
	s, _ := js.Stream(ctx, natsjs.RegistryStream)
	info, _ := s.Info(ctx)
	if info.State.Msgs != 1 {
		t.Fatalf("stream holds %d messages for one record, want 1", info.State.Msgs)
	}
	msg, err := s.GetLastMsgForSubject(ctx, "registry.events.model_version_stage_changed")
	if err != nil {
		t.Fatal(err)
	}
	if msg.Header.Get(natsjs.EventTypeHeader) != rec.Type {
		t.Fatalf("event type header = %q", msg.Header.Get(natsjs.EventTypeHeader))
	}
}
