package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/services/registry/internal/adapters/memory"
	"github.com/Pohi11/spotter/services/registry/internal/adapters/protoevents"
	"github.com/Pohi11/spotter/services/registry/internal/app"
	"github.com/Pohi11/spotter/services/registry/internal/domain"
	"github.com/Pohi11/spotter/services/registry/internal/ports"
	"github.com/Pohi11/spotter/services/registry/internal/registrytest"
)

// flakyPublisher fails after `ok` successful publishes, until healed.
type flakyPublisher struct {
	ok        int
	published []ports.OutboxRecord
	healed    bool
}

func (p *flakyPublisher) Publish(_ context.Context, rec ports.OutboxRecord) error {
	if !p.healed && len(p.published) >= p.ok {
		return errors.New("nats: connection closed")
	}
	p.published = append(p.published, rec)
	return nil
}

func TestRelayKeepsOrderAndRetriesAfterAFailure(t *testing.T) {
	spectest.Verifies(t, "event-streams", "The registry publishes its outbox")
	ctx := context.Background()
	repo := memory.New()
	svc := app.NewService(repo, protoevents.Encoder{}, registrytest.IDs{}, registrytest.NewClock())
	if _, err := svc.RegisterModel(ctx, "eurosat-classifier", ""); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := svc.RegisterVersion(ctx, app.RegisterVersionInput{ModelName: "eurosat-classifier", ArtifactURI: "s3://m/k", ArtifactSHA256: registrytest.SHA, Framework: domain.FrameworkPyTorch}); err != nil {
			t.Fatal(err)
		}
	}
	all, _ := repo.PendingOutbox(ctx, 100)

	pub := &flakyPublisher{ok: 1}
	relay := app.NewRelay(repo, pub, registrytest.NewClock())
	n, err := relay.Once(ctx, 100)
	if n != 1 || err == nil {
		t.Fatalf("first pass published %d, err %v; want 1 and an error", n, err)
	}
	if pending, _ := repo.PendingOutbox(ctx, 100); len(pending) != 2 || pending[0].ID != all[1].ID {
		t.Fatalf("pending after a failure = %d records, want the last 2 in order", len(pending))
	}

	pub.healed = true
	if n, err := relay.Once(ctx, 100); n != 2 || err != nil {
		t.Fatalf("second pass published %d, err %v; want 2, nil", n, err)
	}
	for i, rec := range pub.published {
		if rec.ID != all[i].ID {
			t.Fatalf("published out of order at %d", i)
		}
	}
	if pending, _ := repo.PendingOutbox(ctx, 100); len(pending) != 0 {
		t.Fatalf("%d records still pending", len(pending))
	}
}
