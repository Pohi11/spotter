// Package grpcapi serves spotter.v1.RegistryService over gRPC. It is a driving adapter:
// it parses requests, calls app.Service, and maps results and errors back to protobuf.
package grpcapi

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/services/registry/internal/adapters/protoevents"
	"github.com/Pohi11/spotter/services/registry/internal/app"
	"github.com/Pohi11/spotter/services/registry/internal/domain"
)

// Server implements spotterv1.RegistryServiceServer.
type Server struct {
	spotterv1.UnimplementedRegistryServiceServer
	svc    *app.Service
	logger *slog.Logger
}

// New returns a server backed by svc. Unexpected errors are logged to logger.
func New(svc *app.Service, logger *slog.Logger) *Server {
	return &Server{svc: svc, logger: logger}
}

func (s *Server) RegisterModel(ctx context.Context, req *spotterv1.RegisterModelRequest) (*spotterv1.RegisterModelResponse, error) {
	m, err := s.svc.RegisterModel(ctx, req.GetName(), req.GetDescription())
	if err != nil {
		return nil, s.toStatus(err)
	}
	return &spotterv1.RegisterModelResponse{Model: model(m)}, nil
}

func (s *Server) GetModel(ctx context.Context, req *spotterv1.GetModelRequest) (*spotterv1.GetModelResponse, error) {
	m, err := s.svc.Model(ctx, req.GetName())
	if err != nil {
		return nil, s.toStatus(err)
	}
	return &spotterv1.GetModelResponse{Model: model(m)}, nil
}

func (s *Server) RegisterVersion(ctx context.Context, req *spotterv1.RegisterVersionRequest) (*spotterv1.RegisterVersionResponse, error) {
	v, err := s.svc.RegisterVersion(ctx, app.RegisterVersionInput{
		ModelName:      req.GetModelName(),
		ArtifactURI:    req.GetArtifactUri(),
		ArtifactSHA256: req.GetArtifactSha256(),
		Framework:      domain.Framework(req.GetFramework()),
		Metrics:        req.GetMetrics(),
	})
	if err != nil {
		return nil, s.toStatus(err)
	}
	return &spotterv1.RegisterVersionResponse{Version: version(v)}, nil
}

func (s *Server) ListVersions(ctx context.Context, req *spotterv1.ListVersionsRequest) (*spotterv1.ListVersionsResponse, error) {
	vs, err := s.svc.Versions(ctx, req.GetModelName())
	if err != nil {
		return nil, s.toStatus(err)
	}
	out := make([]*spotterv1.ModelVersion, len(vs))
	for i, v := range vs {
		out[i] = version(v)
	}
	return &spotterv1.ListVersionsResponse{Versions: out}, nil
}

func (s *Server) GetVersion(ctx context.Context, req *spotterv1.GetVersionRequest) (*spotterv1.GetVersionResponse, error) {
	id, err := parseID("version_id", req.GetVersionId())
	if err != nil {
		return nil, s.toStatus(err)
	}
	v, err := s.svc.Version(ctx, id)
	if err != nil {
		return nil, s.toStatus(err)
	}
	return &spotterv1.GetVersionResponse{Version: version(v)}, nil
}

func (s *Server) TransitionVersion(ctx context.Context, req *spotterv1.TransitionVersionRequest) (*spotterv1.TransitionVersionResponse, error) {
	id, err := parseID("version_id", req.GetVersionId())
	if err != nil {
		return nil, s.toStatus(err)
	}
	to, ok := stageFromProto[req.GetToStage()]
	if !ok {
		return nil, s.toStatus(&domain.ValidationError{Field: "to_stage", Problem: "must be a known stage"})
	}
	v, err := s.svc.TransitionVersion(ctx, id, to, req.GetReason(), req.GetExpectedRevision())
	if err != nil {
		return nil, s.toStatus(err)
	}
	return &spotterv1.TransitionVersionResponse{Version: version(v)}, nil
}

func (s *Server) GetVersionHistory(ctx context.Context, req *spotterv1.GetVersionHistoryRequest) (*spotterv1.GetVersionHistoryResponse, error) {
	id, err := parseID("version_id", req.GetVersionId())
	if err != nil {
		return nil, s.toStatus(err)
	}
	changes, err := s.svc.History(ctx, id)
	if err != nil {
		return nil, s.toStatus(err)
	}
	out := make([]*spotterv1.StageChange, len(changes))
	for i, c := range changes {
		out[i] = &spotterv1.StageChange{
			FromStage: protoevents.Stage(c.From),
			ToStage:   protoevents.Stage(c.To),
			Reason:    c.Reason,
			Revision:  c.Revision,
			ChangedAt: timestamppb.New(c.At),
		}
	}
	return &spotterv1.GetVersionHistoryResponse{Changes: out}, nil
}

// toStatus is the single place registry errors become gRPC statuses (design D2).
func (s *Server) toStatus(err error) error {
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		st := status.New(codes.InvalidArgument, ve.Error())
		if withDetail, derr := st.WithDetails(&errdetails.BadRequest{
			FieldViolations: []*errdetails.BadRequest_FieldViolation{{Field: ve.Field, Description: ve.Problem}},
		}); derr == nil {
			st = withDetail
		}
		return st.Err()
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrIllegalTransition), errors.Is(err, domain.ErrProductionOccupied):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrConflict):
		return status.Error(codes.Aborted, err.Error())
	default:
		// Never send internal error text (SQL, hostnames) to clients; log it instead.
		s.logger.Error("registry request failed", "error", err)
		return status.Error(codes.Internal, "internal error")
	}
}

func parseID(field, raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, &domain.ValidationError{Field: field, Problem: "must be a UUID"}
	}
	return id, nil
}

var stageFromProto = func() map[spotterv1.ModelVersionStage]domain.Stage {
	m := map[spotterv1.ModelVersionStage]domain.Stage{}
	for _, s := range domain.Stages() {
		m[protoevents.Stage(s)] = s
	}
	return m
}()

func model(m domain.Model) *spotterv1.Model {
	return &spotterv1.Model{Id: m.ID.String(), Name: m.Name, Description: m.Description, CreatedAt: timestamppb.New(m.CreatedAt)}
}

func version(v domain.ModelVersion) *spotterv1.ModelVersion {
	return &spotterv1.ModelVersion{
		Id:             v.ID.String(),
		ModelId:        v.ModelID.String(),
		Version:        int32(v.Number),
		ArtifactUri:    v.ArtifactURI,
		ArtifactSha256: v.ArtifactSHA256,
		Framework:      string(v.Framework),
		Metrics:        v.Metrics,
		Stage:          protoevents.Stage(v.Stage),
		Revision:       v.Revision,
		CreatedAt:      timestamppb.New(v.CreatedAt),
		UpdatedAt:      timestamppb.New(v.UpdatedAt),
	}
}
