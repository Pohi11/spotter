// Package ports defines the interfaces the registry's use cases depend on.
// Adapters (Postgres, in-memory, protobuf) implement them.
package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Pohi11/spotter/services/registry/internal/domain"
)

// OutboxRecord is an encoded event waiting to be published. The repository stores it
// without interpreting it.
type OutboxRecord struct {
	ID          uuid.UUID // becomes the message id when published
	Type        string    // fully qualified protobuf message name, e.g. "spotter.v1.ModelVersionStageChanged"
	AggregateID uuid.UUID // the model version the event is about
	Payload     []byte
	OccurredAt  time.Time
	// TraceParent is the W3C traceparent active when the record was written ("" if none).
	// Adapters fill it from the context; the publisher attaches it as a header.
	TraceParent string
}

// Repository stores models, versions, history, and outbox records.
//
// Every method that changes state also stores the given outbox records in the same
// transaction: both are saved, or neither is.
type Repository interface {
	// CreateModel stores a new model. Returns domain.ErrAlreadyExists if the name is taken.
	CreateModel(ctx context.Context, m domain.Model) error
	// ModelByName returns domain.ErrNotFound if there is no such model.
	ModelByName(ctx context.Context, name string) (domain.Model, error)
	// ModelByID returns domain.ErrNotFound if there is no such model.
	ModelByID(ctx context.Context, id uuid.UUID) (domain.Model, error)

	// AddVersion assigns the next version number for v.ModelID, stores the version, and
	// stores the records that events returns for the numbered version, all atomically.
	// Returns domain.ErrNotFound if the model doesn't exist.
	AddVersion(ctx context.Context, v domain.ModelVersion, events func(numbered domain.ModelVersion) ([]OutboxRecord, error)) (domain.ModelVersion, error)
	// Version returns domain.ErrNotFound if there is no such version.
	Version(ctx context.Context, id uuid.UUID) (domain.ModelVersion, error)
	// Versions lists a model's versions, highest number first.
	Versions(ctx context.Context, modelID uuid.UUID) ([]domain.ModelVersion, error)

	// SaveTransition stores the transitioned version, its history entry, and the outbox
	// records, only if the stored revision still equals expectedRevision.
	// Returns domain.ErrConflict if it doesn't, domain.ErrProductionOccupied if another
	// version of the model is already in production, domain.ErrNotFound if the version
	// doesn't exist.
	SaveTransition(ctx context.Context, v domain.ModelVersion, expectedRevision int64, change domain.StageChange, records []OutboxRecord) error
	// History lists a version's stage changes, oldest first.
	History(ctx context.Context, versionID uuid.UUID) ([]domain.StageChange, error)

	// PendingOutbox lists unpublished outbox records, oldest first.
	PendingOutbox(ctx context.Context, limit int) ([]OutboxRecord, error)
	// MarkPublished records outbox records as published so they are not listed again.
	MarkPublished(ctx context.Context, ids []uuid.UUID, at time.Time) error
}

// EventPublisher publishes one outbox record to the message bus and returns only after
// the bus has acknowledged it. Publishing the same record twice must be harmless.
type EventPublisher interface {
	Publish(ctx context.Context, rec OutboxRecord) error
}

// EventEncoder turns domain events into outbox records.
type EventEncoder interface {
	Encode(e domain.Event) (OutboxRecord, error)
}

// IDGenerator creates new ids.
type IDGenerator interface {
	NewID() uuid.UUID
}

// Clock tells the time.
type Clock interface {
	Now() time.Time
}
