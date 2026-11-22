package grpcapi_test

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/services/gateway/internal/adapters/grpcapi"
	"github.com/Pohi11/spotter/services/gateway/internal/app"
	"github.com/Pohi11/spotter/services/gateway/internal/domain"
	"github.com/Pohi11/spotter/services/registry/registryfake"
)

// fakeWorker serves ClassifierService and counts calls.
type fakeWorker struct {
	spotterv1.UnimplementedClassifierServiceServer
	calls atomic.Int32
}

func (w *fakeWorker) Classify(_ context.Context, req *spotterv1.ClassifyRequest) (*spotterv1.ClassifyResponse, error) {
	w.calls.Add(1)
	return &spotterv1.ClassifyResponse{Prediction: &spotterv1.Prediction{Label: "Forest", Confidence: 0.97, ModelVersion: 3, ModelVersionId: "v3"}}, nil
}

// noStore fails the test if the synchronous path stores, records, or publishes anything.
type noStore struct{ t *testing.T }

func (n noStore) Put(context.Context, string, []byte) (string, error) {
	n.t.Error("image stored")
	return "", nil
}
func (n noStore) Create(context.Context, domain.Job) error { n.t.Error("job recorded"); return nil }
func (n noStore) Get(context.Context, uuid.UUID) (domain.Job, error) {
	return domain.Job{}, domain.ErrNotFound
}
func (n noStore) MarkFailed(context.Context, uuid.UUID, string, time.Time) error { return nil }
func (n noStore) PublishRequested(context.Context, domain.Job) error {
	n.t.Error("event published")
	return nil
}

func conn(t *testing.T, lis *bufconn.Listener) *grpc.ClientConn {
	t.Helper()
	c, err := grpc.NewClient("passthrough:///x", dialer(lis), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// syncSetup serves the gateway's InferenceService with a fake registry that knows
// eurosat-classifier and, if worker is non-nil, a worker behind WORKER_ADDR.
func syncSetup(t *testing.T, worker spotterv1.ClassifierServiceServer, workerUp bool) spotterv1.InferenceServiceClient {
	t.Helper()
	regLis := bufconn.Listen(1 << 20)
	reg := registryfake.Serve(regLis)
	t.Cleanup(reg.GRPC.Stop)
	registry := spotterv1.NewRegistryServiceClient(conn(t, regLis))
	if _, err := registry.RegisterModel(ctx, &spotterv1.RegisterModelRequest{Name: "eurosat-classifier"}); err != nil {
		t.Fatal(err)
	}
	ns := noStore{t}
	deps := app.Deps{Images: ns, Jobs: ns, Pub: ns, Models: grpcapi.Catalog{Registry: registry}, IDs: uuidV7{}, Clock: clock{}}
	if worker != nil {
		wLis := bufconn.Listen(1 << 20)
		ws := grpc.NewServer()
		spotterv1.RegisterClassifierServiceServer(ws, worker)
		if workerUp {
			go func() { _ = ws.Serve(wLis) }()
			t.Cleanup(ws.Stop)
		} else {
			wLis.Close() // nothing listens: dials fail
		}
		deps.Classifier = grpcapi.Worker{Client: spotterv1.NewClassifierServiceClient(conn(t, wLis))}
	}
	gwLis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer()
	spotterv1.RegisterInferenceServiceServer(gs, grpcapi.New(app.NewService(deps), slog.New(slog.NewTextHandler(io.Discard, nil))))
	go func() { _ = gs.Serve(gwLis) }()
	t.Cleanup(gs.Stop)
	return spotterv1.NewInferenceServiceClient(conn(t, gwLis))
}

const syncReq = "Synchronous classification calls the worker directly"

func TestClassifySyncReturnsTheWorkersPrediction(t *testing.T) {
	spectest.Verifies(t, capability, syncReq)
	w := &fakeWorker{}
	client := syncSetup(t, w, true)
	resp, err := client.ClassifySync(ctx, &spotterv1.ClassifySyncRequest{ModelName: "eurosat-classifier", Image: []byte("jpeg")})
	if err != nil {
		t.Fatal(err)
	}
	if p := resp.GetPrediction(); p.GetLabel() != "Forest" || p.GetModelVersion() != 3 || w.calls.Load() != 1 {
		t.Fatalf("prediction %v, worker calls %d", p, w.calls.Load())
	}
}

func TestClassifySyncWithoutWorkerIsUnimplemented(t *testing.T) {
	spectest.Verifies(t, capability, syncReq)
	client := syncSetup(t, nil, false)
	_, err := client.ClassifySync(ctx, &spotterv1.ClassifySyncRequest{ModelName: "eurosat-classifier", Image: []byte("jpeg")})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("code %v, want Unimplemented", status.Code(err))
	}
}

func TestClassifySyncWithWorkerDownIsUnavailable(t *testing.T) {
	spectest.Verifies(t, capability, syncReq)
	client := syncSetup(t, &fakeWorker{}, false)
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := client.ClassifySync(cctx, &spotterv1.ClassifySyncRequest{ModelName: "eurosat-classifier", Image: []byte("jpeg")})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("code %v (%v), want Unavailable", status.Code(err), err)
	}
}

func TestClassifySyncUnknownModelDoesNotCallTheWorker(t *testing.T) {
	spectest.Verifies(t, capability, syncReq)
	w := &fakeWorker{}
	client := syncSetup(t, w, true)
	_, err := client.ClassifySync(ctx, &spotterv1.ClassifySyncRequest{ModelName: "no-such-model", Image: []byte("jpeg")})
	if status.Code(err) != codes.NotFound || w.calls.Load() != 0 {
		t.Fatalf("code %v, worker calls %d; want NotFound and 0", status.Code(err), w.calls.Load())
	}
}
