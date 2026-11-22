// Package registryfake runs a real registry gRPC server backed by the in-memory
// repository, so other services' tests can talk to a registry without Postgres.
// It lives outside internal/ so those services can import it; production code must not.
package registryfake

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/services/registry/internal/adapters/grpcapi"
	"github.com/Pohi11/spotter/services/registry/internal/adapters/memory"
	"github.com/Pohi11/spotter/services/registry/internal/adapters/protoevents"
	"github.com/Pohi11/spotter/services/registry/internal/app"
	"github.com/Pohi11/spotter/services/registry/internal/registrytest"
)

// Server is a running fake registry.
type Server struct {
	GRPC   *grpc.Server
	Health *health.Server
	svc    *app.Service
}

// RecordShadowComparisons stores n comparisons for a shadow version, the first `agreeing`
// of them agreeing, as the registry's shadow consumer would from ShadowCompared events.
func (s *Server) RecordShadowComparisons(ctx context.Context, versionID string, n, agreeing int, shadowMS float64) error {
	id, err := uuid.Parse(versionID)
	if err != nil {
		return err
	}
	for i := range n {
		c := app.ShadowComparison{ShadowVersionID: id, JobID: fmt.Sprintf("job-%d", i), Agree: i < agreeing, ShadowMS: shadowMS}
		if err := s.svc.RecordShadowComparison(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

// Serve starts a registry on lis (for example a bufconn or a 127.0.0.1:0 listener) and
// returns immediately. Call GRPC.Stop to stop it.
func Serve(lis net.Listener) *Server {
	svc := app.NewService(memory.New(), protoevents.Encoder{}, registrytest.IDs{}, registrytest.NewClock())
	s := grpc.NewServer()
	spotterv1.RegisterRegistryServiceServer(s, grpcapi.New(svc, slog.New(slog.NewTextHandler(io.Discard, nil))))
	h := health.NewServer()
	h.SetServingStatus(spotterv1.RegistryService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(s, h)
	go func() { _ = s.Serve(lis) }()
	return &Server{GRPC: s, Health: h, svc: svc}
}

// SHA is a valid artifact checksum for tests.
const SHA = registrytest.SHA
