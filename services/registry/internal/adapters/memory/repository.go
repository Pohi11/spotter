// Package memory is an in-memory ports.Repository for unit tests. It enforces the same
// rules as the Postgres adapter and passes the same contract suite (registrytest).
package memory

import (
	"context"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Pohi11/spotter/services/registry/internal/domain"
	"github.com/Pohi11/spotter/services/registry/internal/ports"
)

// Repository is safe for concurrent use. Every method holds one mutex for its whole
// duration, which plays the role of a database transaction.
type Repository struct {
	mu        sync.Mutex
	models    map[uuid.UUID]domain.Model
	versions  map[uuid.UUID]domain.ModelVersion
	history   map[uuid.UUID][]domain.StageChange
	outbox    []ports.OutboxRecord
	published map[uuid.UUID]bool
}

// New returns an empty repository.
func New() *Repository {
	return &Repository{
		models:    map[uuid.UUID]domain.Model{},
		versions:  map[uuid.UUID]domain.ModelVersion{},
		history:   map[uuid.UUID][]domain.StageChange{},
		published: map[uuid.UUID]bool{},
	}
}

var _ ports.Repository = (*Repository)(nil)

func (r *Repository) CreateModel(_ context.Context, m domain.Model) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.models {
		if existing.Name == m.Name {
			return domain.ErrAlreadyExists
		}
	}
	r.models[m.ID] = m
	return nil
}

func (r *Repository) ModelByName(_ context.Context, name string) (domain.Model, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range r.models {
		if m.Name == name {
			return m, nil
		}
	}
	return domain.Model{}, domain.ErrNotFound
}

func (r *Repository) ModelByID(_ context.Context, id uuid.UUID) (domain.Model, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.models[id]
	if !ok {
		return domain.Model{}, domain.ErrNotFound
	}
	return m, nil
}

func (r *Repository) AddVersion(_ context.Context, v domain.ModelVersion, events func(domain.ModelVersion) ([]ports.OutboxRecord, error)) (domain.ModelVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.models[v.ModelID]; !ok {
		return domain.ModelVersion{}, domain.ErrNotFound
	}
	highest := 0
	for _, existing := range r.versions {
		if existing.ModelID == v.ModelID && existing.Number > highest {
			highest = existing.Number
		}
	}
	v.Number = highest + 1
	records, err := events(clone(v))
	if err != nil {
		return domain.ModelVersion{}, err // nothing stored yet, so nothing to undo
	}
	r.versions[v.ID] = clone(v)
	r.outbox = append(r.outbox, records...)
	return clone(v), nil
}

func (r *Repository) Version(_ context.Context, id uuid.UUID) (domain.ModelVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.versions[id]
	if !ok {
		return domain.ModelVersion{}, domain.ErrNotFound
	}
	return clone(v), nil
}

func (r *Repository) Versions(_ context.Context, modelID uuid.UUID) ([]domain.ModelVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.ModelVersion
	for _, v := range r.versions {
		if v.ModelID == modelID {
			out = append(out, clone(v))
		}
	}
	slices.SortFunc(out, func(a, b domain.ModelVersion) int { return b.Number - a.Number })
	return out, nil
}

func (r *Repository) SaveTransition(_ context.Context, v domain.ModelVersion, expectedRevision int64, change domain.StageChange, records []ports.OutboxRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	stored, ok := r.versions[v.ID]
	if !ok {
		return domain.ErrNotFound
	}
	if stored.Revision != expectedRevision {
		return domain.ErrConflict
	}
	if v.Stage == domain.StageProduction {
		for id, other := range r.versions {
			if id != v.ID && other.ModelID == v.ModelID && other.Stage == domain.StageProduction {
				return domain.ErrProductionOccupied
			}
		}
	}
	r.versions[v.ID] = clone(v)
	r.history[v.ID] = append(r.history[v.ID], change)
	r.outbox = append(r.outbox, records...)
	return nil
}

func (r *Repository) History(_ context.Context, versionID uuid.UUID) ([]domain.StageChange, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.history[versionID]), nil
}

func (r *Repository) PendingOutbox(_ context.Context, limit int) ([]ports.OutboxRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []ports.OutboxRecord
	for _, rec := range r.outbox {
		if len(out) == limit {
			break
		}
		if r.published[rec.ID] {
			continue
		}
		rec.Payload = slices.Clone(rec.Payload)
		out = append(out, rec)
	}
	return out, nil
}

func (r *Repository) MarkPublished(_ context.Context, ids []uuid.UUID, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		r.published[id] = true
	}
	return nil
}

// clone copies the metrics map so callers never share it with the repository.
func clone(v domain.ModelVersion) domain.ModelVersion {
	v.Metrics = maps.Clone(v.Metrics)
	if v.Metrics == nil {
		v.Metrics = map[string]float64{}
	}
	return v
}
