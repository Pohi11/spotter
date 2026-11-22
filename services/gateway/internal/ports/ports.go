// Package ports defines what the gateway's use cases need from the outside world.
package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Pohi11/spotter/services/gateway/internal/domain"
)

// ImageStore keeps submitted images. Put returns the stored image's URI.
type ImageStore interface {
	Put(ctx context.Context, key string, data []byte) (uri string, err error)
}

// JobStore keeps each job's status.
type JobStore interface {
	// Create stores a new pending job.
	Create(ctx context.Context, job domain.Job) error
	// Get returns domain.ErrNotFound for an unknown job.
	Get(ctx context.Context, id uuid.UUID) (domain.Job, error)
	// MarkFailed records a pending job as failed with a reason.
	MarkFailed(ctx context.Context, id uuid.UUID, reason string, at time.Time) error
}

// JobPublisher announces a new job to the workers.
type JobPublisher interface {
	PublishRequested(ctx context.Context, job domain.Job) error
}

// ModelCatalog answers whether a model exists (the registry).
type ModelCatalog interface {
	ModelExists(ctx context.Context, name string) (bool, error)
}

// IDGenerator creates new ids.
type IDGenerator interface{ NewID() uuid.UUID }

// Clock tells the time.
type Clock interface{ Now() time.Time }

// EventSource reads the platform's event streams.
type EventSource interface {
	// Events sends the events published in the last `since`, in stream order per stream,
	// then live ones, until ctx is done; then it releases its resources and closes the
	// channel.
	Events(ctx context.Context, since time.Duration) (<-chan domain.Event, error)
}
