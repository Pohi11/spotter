// Package app holds the gateway's inference use cases. It depends only on domain and ports.
package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Pohi11/spotter/services/gateway/internal/domain"
	"github.com/Pohi11/spotter/services/gateway/internal/ports"
)

// Service runs the inference use cases.
type Service struct {
	images ports.ImageStore
	jobs   ports.JobStore
	pub    ports.JobPublisher
	models ports.ModelCatalog
	ids    ports.IDGenerator
	clock  ports.Clock
	sync   ports.Classifier // nil unless synchronous classification is configured
}

// Deps are the ports a Service needs.
type Deps struct {
	Images ports.ImageStore
	Jobs   ports.JobStore
	Pub    ports.JobPublisher
	Models ports.ModelCatalog
	IDs    ports.IDGenerator
	Clock  ports.Clock
	// Classifier is optional: without it, ClassifySync answers ErrNotConfigured.
	Classifier ports.Classifier
}

func NewService(d Deps) *Service {
	return &Service{images: d.Images, jobs: d.Jobs, pub: d.Pub, models: d.Models, ids: d.IDs, clock: d.Clock, sync: d.Classifier}
}

// Submit validates the request, stores the image, records a pending job, and publishes
// it for the workers. It returns as soon as the job is queued.
func (s *Service) Submit(ctx context.Context, modelName string, image []byte) (domain.Job, error) {
	if err := domain.ValidateSubmission(modelName, image); err != nil {
		return domain.Job{}, err
	}
	exists, err := s.models.ModelExists(ctx, modelName)
	if err != nil {
		return domain.Job{}, fmt.Errorf("look up model %q: %w: %w", modelName, domain.ErrUnavailable, err)
	}
	if !exists {
		return domain.Job{}, fmt.Errorf("model %q: %w", modelName, domain.ErrNotFound)
	}

	id := s.ids.NewID()
	uri, err := s.images.Put(ctx, modelName+"/"+id.String(), image)
	if err != nil {
		return domain.Job{}, fmt.Errorf("store image: %w: %w", domain.ErrUnavailable, err)
	}
	job := domain.Job{ID: id, ModelName: modelName, Status: domain.StatusPending, ImageURI: uri, SubmittedAt: s.clock.Now()}
	if err := s.jobs.Create(ctx, job); err != nil {
		return domain.Job{}, fmt.Errorf("record job: %w: %w", domain.ErrUnavailable, err)
	}
	if err := s.pub.PublishRequested(ctx, job); err != nil {
		// The job can't reach a worker; don't leave it pending forever.
		_ = s.jobs.MarkFailed(ctx, id, "could not queue the job", s.clock.Now())
		return domain.Job{}, fmt.Errorf("queue job: %w: %w", domain.ErrUnavailable, err)
	}
	return job, nil
}

// Get returns a job's current status and result.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (domain.Job, error) {
	return s.jobs.Get(ctx, id)
}

// ClassifySync validates the request, checks the model, and asks the worker directly,
// storing nothing and publishing nothing (the failure lab's synchronous baseline).
func (s *Service) ClassifySync(ctx context.Context, modelName string, image []byte) (domain.Prediction, error) {
	if err := domain.ValidateSubmission(modelName, image); err != nil {
		return domain.Prediction{}, err
	}
	if s.sync == nil {
		return domain.Prediction{}, fmt.Errorf("synchronous classification: %w", domain.ErrNotConfigured)
	}
	exists, err := s.models.ModelExists(ctx, modelName)
	if err != nil {
		return domain.Prediction{}, fmt.Errorf("look up model %q: %w: %w", modelName, domain.ErrUnavailable, err)
	}
	if !exists {
		return domain.Prediction{}, fmt.Errorf("model %q: %w", modelName, domain.ErrNotFound)
	}
	p, err := s.sync.Classify(ctx, modelName, image)
	if err != nil {
		return domain.Prediction{}, fmt.Errorf("classify: %w", err)
	}
	return p, nil
}
