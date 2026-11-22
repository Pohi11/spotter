// Package app holds the registry's use cases. It depends only on domain and ports.
package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Pohi11/spotter/services/registry/internal/domain"
	"github.com/Pohi11/spotter/services/registry/internal/ports"
)

// Service runs the registry's use cases.
type Service struct {
	repo  ports.Repository
	enc   ports.EventEncoder
	ids   ports.IDGenerator
	clock ports.Clock
}

// NewService wires the use cases to their ports.
func NewService(repo ports.Repository, enc ports.EventEncoder, ids ports.IDGenerator, clock ports.Clock) *Service {
	return &Service{repo: repo, enc: enc, ids: ids, clock: clock}
}

// RegisterModel creates a model with a unique name.
func (s *Service) RegisterModel(ctx context.Context, name, description string) (domain.Model, error) {
	m, err := domain.NewModel(s.ids.NewID(), name, description, s.clock.Now())
	if err != nil {
		return domain.Model{}, err
	}
	if err := s.repo.CreateModel(ctx, m); err != nil {
		return domain.Model{}, fmt.Errorf("register model %q: %w", name, err)
	}
	return m, nil
}

// RegisterVersionInput describes a new version.
type RegisterVersionInput struct {
	ModelName      string
	ArtifactURI    string
	ArtifactSHA256 string
	Framework      domain.Framework
	Metrics        map[string]float64
}

// RegisterVersion stores a new version of a model, numbered after the model's latest,
// together with a VersionRegistered event.
func (s *Service) RegisterVersion(ctx context.Context, in RegisterVersionInput) (domain.ModelVersion, error) {
	m, err := s.repo.ModelByName(ctx, in.ModelName)
	if err != nil {
		return domain.ModelVersion{}, fmt.Errorf("register version of %q: %w", in.ModelName, err)
	}
	v, err := domain.NewVersion(s.ids.NewID(), m.ID, in.ArtifactURI, in.ArtifactSHA256, in.Framework, in.Metrics, s.clock.Now())
	if err != nil {
		return domain.ModelVersion{}, err
	}
	eventID := s.ids.NewID()
	stored, err := s.repo.AddVersion(ctx, v, func(numbered domain.ModelVersion) ([]ports.OutboxRecord, error) {
		rec, err := s.enc.Encode(domain.VersionRegistered{ID: eventID, ModelName: m.Name, Version: numbered})
		if err != nil {
			return nil, err
		}
		return []ports.OutboxRecord{rec}, nil
	})
	if err != nil {
		return domain.ModelVersion{}, fmt.Errorf("register version of %q: %w", in.ModelName, err)
	}
	return stored, nil
}

// TransitionVersion moves a version to another stage. expectedRevision is the revision
// the caller last read; if the version has changed since, the call fails with
// domain.ErrConflict and changes nothing.
func (s *Service) TransitionVersion(ctx context.Context, versionID uuid.UUID, to domain.Stage, reason string, expectedRevision int64) (domain.ModelVersion, error) {
	v, err := s.repo.Version(ctx, versionID)
	if err != nil {
		return domain.ModelVersion{}, fmt.Errorf("transition version %s: %w", versionID, err)
	}
	if v.Revision != expectedRevision {
		return domain.ModelVersion{}, fmt.Errorf("transition version %s: expected revision %d, stored %d: %w", versionID, expectedRevision, v.Revision, domain.ErrConflict)
	}
	next, change, err := v.Transition(to, reason, s.clock.Now())
	if err != nil {
		return domain.ModelVersion{}, err
	}
	m, err := s.repo.ModelByID(ctx, v.ModelID)
	if err != nil {
		return domain.ModelVersion{}, fmt.Errorf("transition version %s: %w", versionID, err)
	}
	rec, err := s.enc.Encode(domain.StageChanged{ID: s.ids.NewID(), ModelID: m.ID, ModelName: m.Name, Number: v.Number, Change: change})
	if err != nil {
		return domain.ModelVersion{}, err
	}
	// The repository re-checks the revision inside its transaction: the check above is
	// only a fast path, and two callers can both pass it.
	if err := s.repo.SaveTransition(ctx, next, expectedRevision, change, []ports.OutboxRecord{rec}); err != nil {
		return domain.ModelVersion{}, fmt.Errorf("transition version %s to %s: %w", versionID, to, err)
	}
	return next, nil
}

// Version returns one version.
func (s *Service) Version(ctx context.Context, id uuid.UUID) (domain.ModelVersion, error) {
	return s.repo.Version(ctx, id)
}

// Versions lists a model's versions, newest first.
func (s *Service) Versions(ctx context.Context, modelName string) ([]domain.ModelVersion, error) {
	m, err := s.repo.ModelByName(ctx, modelName)
	if err != nil {
		return nil, fmt.Errorf("list versions of %q: %w", modelName, err)
	}
	return s.repo.Versions(ctx, m.ID)
}

// History lists a version's stage changes, oldest first.
func (s *Service) History(ctx context.Context, versionID uuid.UUID) ([]domain.StageChange, error) {
	if _, err := s.repo.Version(ctx, versionID); err != nil {
		return nil, err
	}
	return s.repo.History(ctx, versionID)
}
