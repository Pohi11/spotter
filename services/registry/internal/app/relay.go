package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Pohi11/spotter/services/registry/internal/ports"
)

// Relay publishes the outbox: the second half of the transactional outbox pattern.
type Relay struct {
	repo  ports.Repository
	pub   ports.EventPublisher
	clock ports.Clock
}

func NewRelay(repo ports.Repository, pub ports.EventPublisher, clock ports.Clock) *Relay {
	return &Relay{repo: repo, pub: pub, clock: clock}
}

// Once publishes up to limit pending records in order and marks the published ones.
// It stops at the first failure so records are never published out of order; the rest
// are retried on the next call. A crash after publishing but before marking means the
// record is published again later, which the bus de-duplicates by record id.
func (r *Relay) Once(ctx context.Context, limit int) (int, error) {
	recs, err := r.repo.PendingOutbox(ctx, limit)
	if err != nil {
		return 0, fmt.Errorf("read outbox: %w", err)
	}
	var done []uuid.UUID
	var pubErr error
	for _, rec := range recs {
		if err := r.pub.Publish(ctx, rec); err != nil {
			pubErr = fmt.Errorf("publish %s %s: %w", rec.Type, rec.ID, err)
			break
		}
		done = append(done, rec.ID)
	}
	if len(done) > 0 {
		if err := r.repo.MarkPublished(ctx, done, r.clock.Now()); err != nil {
			return 0, fmt.Errorf("mark published: %w", err)
		}
	}
	return len(done), pubErr
}
