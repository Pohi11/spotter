package domain

import (
	"time"

	"github.com/google/uuid"
)

// Event is something that happened in the registry, stored in the outbox in the same
// transaction as the change, and later published to NATS. The protobuf shape of each
// event lives in proto/spotter/v1/registry_events.proto; an adapter does the encoding.
type Event interface {
	EventID() uuid.UUID
	AggregateID() uuid.UUID // the model version the event is about
	OccurredAt() time.Time
}

// VersionRegistered is recorded when a new version is stored.
type VersionRegistered struct {
	ID        uuid.UUID
	ModelName string
	Version   ModelVersion
}

func (e VersionRegistered) EventID() uuid.UUID     { return e.ID }
func (e VersionRegistered) AggregateID() uuid.UUID { return e.Version.ID }
func (e VersionRegistered) OccurredAt() time.Time  { return e.Version.CreatedAt }

// StageChanged is recorded when a version moves to another stage.
type StageChanged struct {
	ID        uuid.UUID
	ModelID   uuid.UUID
	ModelName string
	Number    int
	Change    StageChange
}

func (e StageChanged) EventID() uuid.UUID     { return e.ID }
func (e StageChanged) AggregateID() uuid.UUID { return e.Change.VersionID }
func (e StageChanged) OccurredAt() time.Time  { return e.Change.At }
