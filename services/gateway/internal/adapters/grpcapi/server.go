// Package grpcapi serves spotter.v1.InferenceService (a driving adapter), and looks
// models up in the registry (a driven adapter for ports.ModelCatalog).
package grpcapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/services/gateway/internal/adapters/natsjobs"
	"github.com/Pohi11/spotter/services/gateway/internal/app"
	"github.com/Pohi11/spotter/services/gateway/internal/domain"
)

// Server implements spotterv1.InferenceServiceServer.
type Server struct {
	spotterv1.UnimplementedInferenceServiceServer
	svc    *app.Service
	logger *slog.Logger
}

func New(svc *app.Service, logger *slog.Logger) *Server { return &Server{svc: svc, logger: logger} }

func (s *Server) SubmitInference(ctx context.Context, req *spotterv1.SubmitInferenceRequest) (*spotterv1.SubmitInferenceResponse, error) {
	job, err := s.svc.Submit(ctx, req.GetModelName(), req.GetImage())
	if err != nil {
		return nil, s.toStatus(err)
	}
	return &spotterv1.SubmitInferenceResponse{JobId: job.ID.String()}, nil
}

func (s *Server) GetInference(ctx context.Context, req *spotterv1.GetInferenceRequest) (*spotterv1.GetInferenceResponse, error) {
	id, err := uuid.Parse(req.GetJobId())
	if err != nil {
		return nil, s.toStatus(&domain.ValidationError{Field: "job_id", Problem: "must be a UUID"})
	}
	job, err := s.svc.Get(ctx, id)
	if err != nil {
		return nil, s.toStatus(err)
	}
	return &spotterv1.GetInferenceResponse{Job: natsjobs.ToProto(job)}, nil
}

func (s *Server) ClassifySync(ctx context.Context, req *spotterv1.ClassifySyncRequest) (*spotterv1.ClassifySyncResponse, error) {
	p, err := s.svc.ClassifySync(ctx, req.GetModelName(), req.GetImage())
	if err != nil {
		return nil, s.toStatus(err)
	}
	return &spotterv1.ClassifySyncResponse{Prediction: natsjobs.PredictionToProto(p)}, nil
}

func (s *Server) toStatus(err error) error {
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		st := status.New(codes.InvalidArgument, ve.Error())
		if d, derr := st.WithDetails(&errdetails.BadRequest{FieldViolations: []*errdetails.BadRequest_FieldViolation{{Field: ve.Field, Description: ve.Problem}}}); derr == nil {
			st = d
		}
		return st.Err()
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrNotConfigured):
		return status.Error(codes.Unimplemented, "synchronous classification is not configured on this gateway (WORKER_ADDR)")
	case errors.Is(err, domain.ErrUnavailable):
		s.logger.Warn("inference request failed", "error", err)
		return status.Error(codes.Unavailable, "a dependency is unavailable; retry later")
	default:
		s.logger.Error("inference request failed", "error", err)
		return status.Error(codes.Internal, "internal error")
	}
}

// Catalog implements ports.ModelCatalog with the registry's GetModel.
type Catalog struct {
	Registry spotterv1.RegistryServiceClient
}

func (c Catalog) ModelExists(ctx context.Context, name string) (bool, error) {
	_, err := c.Registry.GetModel(ctx, &spotterv1.GetModelRequest{Name: name})
	switch status.Code(err) {
	case codes.OK:
		return true, nil
	case codes.NotFound:
		return false, nil
	default:
		return false, err
	}
}

// Worker implements ports.Classifier with the worker's ClassifierService.
type Worker struct {
	Client spotterv1.ClassifierServiceClient
}

func (w Worker) Classify(ctx context.Context, modelName string, image []byte) (domain.Prediction, error) {
	resp, err := w.Client.Classify(ctx, &spotterv1.ClassifyRequest{ModelName: modelName, Image: image})
	if err != nil {
		return domain.Prediction{}, fmt.Errorf("worker: %w: %w", domain.ErrUnavailable, err)
	}
	return natsjobs.PredictionFromProto(resp.GetPrediction()), nil
}
