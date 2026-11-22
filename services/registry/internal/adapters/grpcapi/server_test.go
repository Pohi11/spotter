package grpcapi_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/services/registry/internal/adapters/grpcapi"
	"github.com/Pohi11/spotter/services/registry/internal/adapters/memory"
	"github.com/Pohi11/spotter/services/registry/internal/adapters/protoevents"
	"github.com/Pohi11/spotter/services/registry/internal/app"
	"github.com/Pohi11/spotter/services/registry/internal/domain"
	"github.com/Pohi11/spotter/services/registry/internal/ports"
	"github.com/Pohi11/spotter/services/registry/internal/registrytest"
)

const capability = "registry-api"

var ctx = context.Background()

// client serves the registry over an in-memory listener and returns a client for it.
func client(t *testing.T, repo ports.Repository) spotterv1.RegistryServiceClient {
	t.Helper()
	svc := app.NewService(repo, protoevents.Encoder{}, registrytest.IDs{}, registrytest.NewClock())
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	spotterv1.RegisterRegistryServiceServer(srv, grpcapi.New(svc, slog.New(slog.NewTextHandler(io.Discard, nil))))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return spotterv1.NewRegistryServiceClient(conn)
}

func registerVersion(t *testing.T, c spotterv1.RegistryServiceClient) *spotterv1.ModelVersion {
	t.Helper()
	if _, err := c.RegisterModel(ctx, &spotterv1.RegisterModelRequest{Name: "eurosat-classifier"}); err != nil &&
		status.Code(err) != codes.AlreadyExists {
		t.Fatal(err)
	}
	resp, err := c.RegisterVersion(ctx, &spotterv1.RegisterVersionRequest{
		ModelName: "eurosat-classifier", ArtifactUri: "s3://models/eurosat/v1.pt",
		ArtifactSha256: registrytest.SHA, Framework: "pytorch", Metrics: map[string]float64{"accuracy": 0.91},
	})
	if err != nil {
		t.Fatal(err)
	}
	return resp.GetVersion()
}

func wantCode(t *testing.T, err error, code codes.Code) *status.Status {
	t.Helper()
	st, ok := status.FromError(err)
	if !ok || st.Code() != code {
		t.Fatalf("err = %v, want %s", err, code)
	}
	return st
}

func wantFieldViolation(t *testing.T, err error, field string) {
	t.Helper()
	st := wantCode(t, err, codes.InvalidArgument)
	for _, d := range st.Details() {
		if br, ok := d.(*errdetails.BadRequest); ok {
			for _, fv := range br.GetFieldViolations() {
				if fv.GetField() == field {
					return
				}
			}
		}
	}
	t.Fatalf("no field violation for %q in %v", field, st.Details())
}

func TestRegisterAndReadBack(t *testing.T) {
	spectest.Verifies(t, capability, "Registry operations are available over gRPC")
	c := client(t, memory.New())
	v := registerVersion(t, c)
	got, err := c.GetVersion(ctx, &spotterv1.GetVersionRequest{VersionId: v.GetId()})
	if err != nil {
		t.Fatal(err)
	}
	g := got.GetVersion()
	if g.GetVersion() != 1 || g.GetStage() != spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_REGISTERED || g.GetRevision() != 1 ||
		g.GetArtifactUri() != "s3://models/eurosat/v1.pt" || g.GetMetrics()["accuracy"] != 0.91 {
		t.Fatalf("version = %v", g)
	}
	m, err := c.GetModel(ctx, &spotterv1.GetModelRequest{Name: "eurosat-classifier"})
	if err != nil || m.GetModel().GetId() != g.GetModelId() {
		t.Fatalf("get model: %v, %v", m, err)
	}
}

func TestTransitionAndHistory(t *testing.T) {
	spectest.Verifies(t, capability, "Registry operations are available over gRPC")
	c := client(t, memory.New())
	v := registerVersion(t, c)
	resp, err := c.TransitionVersion(ctx, &spotterv1.TransitionVersionRequest{
		VersionId: v.GetId(), ToStage: spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_VALIDATED,
		Reason: "holdout accuracy 0.91", ExpectedRevision: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetVersion().GetRevision() != 2 || resp.GetVersion().GetStage() != spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_VALIDATED {
		t.Fatalf("transitioned version = %v", resp.GetVersion())
	}
	hist, err := c.GetVersionHistory(ctx, &spotterv1.GetVersionHistoryRequest{VersionId: v.GetId()})
	if err != nil {
		t.Fatal(err)
	}
	ch := hist.GetChanges()
	if len(ch) != 1 || ch[0].GetFromStage() != spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_REGISTERED ||
		ch[0].GetToStage() != spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_VALIDATED || ch[0].GetReason() != "holdout accuracy 0.91" {
		t.Fatalf("history = %v", ch)
	}
}

func TestListVersions(t *testing.T) {
	spectest.Verifies(t, capability, "Registry operations are available over gRPC")
	c := client(t, memory.New())
	for range 3 {
		registerVersion(t, c)
	}
	resp, err := c.ListVersions(ctx, &spotterv1.ListVersionsRequest{ModelName: "eurosat-classifier"})
	if err != nil {
		t.Fatal(err)
	}
	vs := resp.GetVersions()
	if len(vs) != 3 || vs[0].GetVersion() != 3 || vs[2].GetVersion() != 1 {
		t.Fatalf("versions = %v", vs)
	}
}

func TestErrorCodes(t *testing.T) {
	c := client(t, memory.New())
	v := registerVersion(t, c)

	t.Run("not found", func(t *testing.T) {
		spectest.Verifies(t, capability, "Registry errors map to gRPC status codes")
		_, err := c.GetVersion(ctx, &spotterv1.GetVersionRequest{VersionId: uuid.NewString()})
		wantCode(t, err, codes.NotFound)
	})
	t.Run("already exists", func(t *testing.T) {
		spectest.Verifies(t, capability, "Registry errors map to gRPC status codes")
		_, err := c.RegisterModel(ctx, &spotterv1.RegisterModelRequest{Name: "eurosat-classifier"})
		wantCode(t, err, codes.AlreadyExists)
	})
	t.Run("illegal transition", func(t *testing.T) {
		spectest.Verifies(t, capability, "Registry errors map to gRPC status codes")
		_, err := c.TransitionVersion(ctx, &spotterv1.TransitionVersionRequest{
			VersionId: v.GetId(), ToStage: spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_SHADOW, Reason: "skip", ExpectedRevision: 1,
		})
		wantCode(t, err, codes.FailedPrecondition)
	})
	t.Run("stale revision", func(t *testing.T) {
		spectest.Verifies(t, capability, "Registry errors map to gRPC status codes")
		_, err := c.TransitionVersion(ctx, &spotterv1.TransitionVersionRequest{
			VersionId: v.GetId(), ToStage: spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_VALIDATED, Reason: "late", ExpectedRevision: 7,
		})
		wantCode(t, err, codes.Aborted)
	})
}

func TestGenericTransitionToProductionRefused(t *testing.T) {
	spectest.Verifies(t, "model-lifecycle", "Production is entered only through promotion or rollback")
	c := client(t, memory.New())
	v := registerVersion(t, c)
	_, err := c.TransitionVersion(ctx, &spotterv1.TransitionVersionRequest{
		VersionId: v.GetId(), ToStage: spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_PRODUCTION, Reason: "skip", ExpectedRevision: 1,
	})
	st := wantCode(t, err, codes.FailedPrecondition)
	if !strings.Contains(st.Message(), "PromoteVersion") {
		t.Fatalf("message %q does not name PromoteVersion", st.Message())
	}
}

// sentinelRepo fails every model lookup with a fixed domain error.
type sentinelRepo struct {
	ports.Repository
	err error
}

func (r sentinelRepo) ModelByName(context.Context, string) (domain.Model, error) {
	return domain.Model{}, r.err
}

func TestPromotionErrorsAreFailedPrecondition(t *testing.T) {
	for _, err := range []error{domain.ErrProductionOccupied, domain.ErrGatesFailed, domain.ErrNothingToRollBack, domain.ErrPromotionRequired} {
		t.Run(err.Error(), func(t *testing.T) {
			spectest.Verifies(t, capability, "Registry errors map to gRPC status codes")
			c := client(t, sentinelRepo{Repository: memory.New(), err: err})
			_, got := c.GetModel(ctx, &spotterv1.GetModelRequest{Name: "eurosat-classifier"})
			wantCode(t, got, codes.FailedPrecondition)
		})
	}
}

// failingRepo returns an error that must not reach the client.
type failingRepo struct{ ports.Repository }

func (failingRepo) ModelByName(context.Context, string) (domain.Model, error) {
	return domain.Model{}, errors.New("dial tcp 10.0.0.5:5432: connection refused")
}

func TestUnexpectedErrorIsInternalAndHidden(t *testing.T) {
	spectest.Verifies(t, capability, "Registry errors map to gRPC status codes")
	c := client(t, failingRepo{Repository: memory.New()})
	_, err := c.GetModel(ctx, &spotterv1.GetModelRequest{Name: "eurosat-classifier"})
	st := wantCode(t, err, codes.Internal)
	if strings.Contains(st.Message(), "10.0.0.5") || strings.Contains(st.Message(), "refused") {
		t.Fatalf("internal details leaked: %q", st.Message())
	}
}

func TestInvalidInputNamesTheField(t *testing.T) {
	c := client(t, memory.New())
	v := registerVersion(t, c)

	t.Run("malformed checksum", func(t *testing.T) {
		spectest.Verifies(t, capability, "Invalid input names the offending field")
		_, err := c.RegisterVersion(ctx, &spotterv1.RegisterVersionRequest{
			ModelName: "eurosat-classifier", ArtifactUri: "s3://m/k", ArtifactSha256: "abc123", Framework: "pytorch",
		})
		wantFieldViolation(t, err, "artifact_sha256")
	})
	t.Run("unspecified stage", func(t *testing.T) {
		spectest.Verifies(t, capability, "Invalid input names the offending field")
		_, err := c.TransitionVersion(ctx, &spotterv1.TransitionVersionRequest{VersionId: v.GetId(), Reason: "x", ExpectedRevision: 1})
		wantFieldViolation(t, err, "to_stage")
	})
	t.Run("malformed id", func(t *testing.T) {
		spectest.Verifies(t, capability, "Invalid input names the offending field")
		_, err := c.GetVersion(ctx, &spotterv1.GetVersionRequest{VersionId: "not-a-uuid"})
		wantFieldViolation(t, err, "version_id")
	})
}

func TestPromotionOverGRPC(t *testing.T) {
	spectest.Verifies(t, "model-promotion", "Promotion operations are available over gRPC and REST")
	repo := memory.New()
	c := client(t, repo)
	toShadow := func() *spotterv1.ModelVersion {
		v := registerVersion(t, c)
		rev := v.GetRevision()
		for _, s := range []spotterv1.ModelVersionStage{spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_VALIDATED, spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_SHADOW} {
			resp, err := c.TransitionVersion(ctx, &spotterv1.TransitionVersionRequest{VersionId: v.GetId(), ToStage: s, Reason: "test", ExpectedRevision: rev})
			if err != nil {
				t.Fatal(err)
			}
			rev = resp.GetVersion().GetRevision()
		}
		id := uuid.MustParse(v.GetId())
		for i := range 250 {
			if err := repo.RecordShadowComparison(ctx, id, fmt.Sprintf("job-%d", i), true, 3); err != nil {
				t.Fatal(err)
			}
		}
		return v
	}
	promote := func(v *spotterv1.ModelVersion) *spotterv1.PromoteVersionResponse {
		gates, err := c.GetPromotionGates(ctx, &spotterv1.GetPromotionGatesRequest{VersionId: v.GetId()})
		if err != nil || gates.GetAllPassed() {
			t.Fatalf("gates before approval: %v, %v", gates, err)
		}
		_, err = c.PromoteVersion(ctx, &spotterv1.PromoteVersionRequest{VersionId: v.GetId()})
		if st := wantCode(t, err, codes.FailedPrecondition); !strings.Contains(st.Message(), "approval") {
			t.Fatalf("refusal %q does not name the approval gate", st.Message())
		}
		if _, err := c.ApproveVersion(ctx, &spotterv1.ApproveVersionRequest{VersionId: v.GetId(), Approver: "ana", Reason: "agreement 100%"}); err != nil {
			t.Fatal(err)
		}
		resp, err := c.PromoteVersion(ctx, &spotterv1.PromoteVersionRequest{VersionId: v.GetId()})
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	first := promote(toShadow())
	if first.GetRetired() != nil || first.GetPromoted().GetStage() != spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_PRODUCTION {
		t.Fatalf("first promotion: %v", first)
	}
	second := promote(toShadow())
	if second.GetRetired().GetId() != first.GetPromoted().GetId() {
		t.Fatalf("second promotion retired %v", second.GetRetired())
	}
	rb, err := c.RollbackModel(ctx, &spotterv1.RollbackModelRequest{ModelName: "eurosat-classifier", Reason: "regression"})
	if err != nil {
		t.Fatal(err)
	}
	if rb.GetRestored().GetId() != first.GetPromoted().GetId() || rb.GetRolledBack().GetStage() != spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_ROLLED_BACK {
		t.Fatalf("rollback: %v", rb)
	}
}
