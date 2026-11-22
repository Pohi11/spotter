package grpcapi_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/natsjs"
	"github.com/Pohi11/spotter/internal/natstest"
	"github.com/Pohi11/spotter/internal/s3test"
	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/services/gateway/internal/adapters/grpcapi"
	"github.com/Pohi11/spotter/services/gateway/internal/adapters/natsjobs"
	"github.com/Pohi11/spotter/services/gateway/internal/adapters/s3images"
	"github.com/Pohi11/spotter/services/gateway/internal/app"
	"github.com/Pohi11/spotter/services/registry/registryfake"
)

const capability = "inference-api"

var ctx = context.Background()

type env struct {
	client spotterv1.InferenceServiceClient
	js     jetstream.JetStream
	jobs   *natsjobs.Store
	images *s3images.Store
}

func dialer(lis *bufconn.Listener) grpc.DialOption {
	return grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() })
}

// setup wires the real adapters: embedded JetStream, SeaweedFS, and a fake registry that
// knows eurosat-classifier.
func setup(t *testing.T) env {
	t.Helper()
	_, js := natstest.Start(t).Connect(t)
	if err := natsjs.Ensure(ctx, js); err != nil {
		t.Fatal(err)
	}
	jobs, err := natsjobs.New(ctx, js)
	if err != nil {
		t.Fatal(err)
	}
	st := s3test.Start(t)
	images := s3images.New(s3images.Config{Endpoint: st.Endpoint, AccessKey: st.AccessKey, SecretKey: st.SecretKey, Region: st.Region, Bucket: s3test.BucketName()})
	if err := images.EnsureBucket(ctx); err != nil {
		t.Fatal(err)
	}

	regLis := bufconn.Listen(1 << 20)
	reg := registryfake.Serve(regLis)
	t.Cleanup(reg.GRPC.Stop)
	regConn, err := grpc.NewClient("passthrough:///registry", dialer(regLis), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { regConn.Close() })
	registry := spotterv1.NewRegistryServiceClient(regConn)
	if _, err := registry.RegisterModel(ctx, &spotterv1.RegisterModelRequest{Name: "eurosat-classifier"}); err != nil {
		t.Fatal(err)
	}

	svc := app.NewService(app.Deps{Images: images, Jobs: jobs, Pub: jobs, Models: grpcapi.Catalog{Registry: registry}, IDs: uuidV7{}, Clock: clock{}})
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	spotterv1.RegisterInferenceServiceServer(srv, grpcapi.New(svc, slog.New(slog.NewTextHandler(io.Discard, nil))))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient("passthrough:///gateway", dialer(lis), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return env{client: spotterv1.NewInferenceServiceClient(conn), js: js, jobs: jobs, images: images}
}

type uuidV7 struct{}

func (uuidV7) NewID() uuid.UUID { return uuid.Must(uuid.NewV7()) }

type clock struct{}

func (clock) Now() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }

func requested(t *testing.T, js jetstream.JetStream, model string) []*spotterv1.InferenceRequested {
	t.Helper()
	cons, err := js.OrderedConsumer(ctx, natsjs.InferenceStream, jetstream.OrderedConsumerConfig{FilterSubjects: []string{natsjs.InferenceRequestedSubject(model)}})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := cons.FetchNoWait(100)
	if err != nil {
		t.Fatal(err)
	}
	var out []*spotterv1.InferenceRequested
	for msg := range batch.Messages() {
		var r spotterv1.InferenceRequested
		if err := proto.Unmarshal(msg.Data(), &r); err != nil {
			t.Fatal(err)
		}
		out = append(out, &r)
	}
	return out
}

func TestSubmitAnImage(t *testing.T) {
	spectest.Verifies(t, capability, "Images are submitted as asynchronous jobs")
	e := setup(t)
	resp, err := e.client.SubmitInference(ctx, &spotterv1.SubmitInferenceRequest{ModelName: "eurosat-classifier", Image: []byte("\xff\xd8 jpeg bytes")})
	if err != nil {
		t.Fatal(err)
	}
	got, err := e.client.GetInference(ctx, &spotterv1.GetInferenceRequest{JobId: resp.GetJobId()})
	if err != nil {
		t.Fatal(err)
	}
	if got.GetJob().GetStatus() != spotterv1.JobStatus_JOB_STATUS_PENDING {
		t.Fatalf("job = %v, want PENDING", got.GetJob())
	}
	stored, err := e.images.Get(ctx, "eurosat-classifier/"+resp.GetJobId())
	if err != nil || string(stored) != "\xff\xd8 jpeg bytes" {
		t.Fatalf("stored image = %q, %v", stored, err)
	}
	reqs := requested(t, e.js, "eurosat-classifier")
	if len(reqs) != 1 || reqs[0].GetJobId() != resp.GetJobId() || !strings.HasSuffix(reqs[0].GetImageUri(), resp.GetJobId()) {
		t.Fatalf("requested events = %v", reqs)
	}
}

func TestSubmissionsAreValidated(t *testing.T) {
	e := setup(t)
	for _, tc := range []struct {
		name  string
		req   *spotterv1.SubmitInferenceRequest
		code  codes.Code
		field string
	}{
		{"empty image", &spotterv1.SubmitInferenceRequest{ModelName: "eurosat-classifier"}, codes.InvalidArgument, "image"},
		{"oversized image", &spotterv1.SubmitInferenceRequest{ModelName: "eurosat-classifier", Image: make([]byte, 2<<20)}, codes.InvalidArgument, "image"},
		{"unknown model", &spotterv1.SubmitInferenceRequest{ModelName: "no-such-model", Image: []byte("x")}, codes.NotFound, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spectest.Verifies(t, capability, "Submissions are validated")
			_, err := e.client.SubmitInference(ctx, tc.req, grpc.MaxCallSendMsgSize(4<<20))
			st, _ := status.FromError(err)
			if st.Code() != tc.code {
				t.Fatalf("err = %v, want %s", err, tc.code)
			}
			if tc.field != "" {
				found := false
				for _, d := range st.Details() {
					if br, ok := d.(*errdetails.BadRequest); ok && br.GetFieldViolations()[0].GetField() == tc.field {
						found = true
					}
				}
				if !found {
					t.Fatalf("no field violation for %s: %v", tc.field, st.Details())
				}
			}
		})
	}
	if n := len(requested(t, e.js, "eurosat-classifier")) + len(requested(t, e.js, "no-such-model")); n != 0 {
		t.Fatalf("%d requests were published for rejected submissions", n)
	}
}

func TestReadJobStatus(t *testing.T) {
	e := setup(t)
	resp, err := e.client.SubmitInference(ctx, &spotterv1.SubmitInferenceRequest{ModelName: "eurosat-classifier", Image: []byte("x")})
	if err != nil {
		t.Fatal(err)
	}
	t.Run("completed job", func(t *testing.T) {
		spectest.Verifies(t, capability, "Job status can be read")
		// Do what a worker does: record a result on the KV entry.
		kv, _ := e.js.KeyValue(ctx, natsjs.JobsBucket)
		entry, _ := kv.Get(ctx, resp.GetJobId())
		var job spotterv1.InferenceJob
		_ = proto.Unmarshal(entry.Value(), &job)
		job.Status = spotterv1.JobStatus_JOB_STATUS_SUCCEEDED
		job.Prediction = &spotterv1.Prediction{Label: "Forest", Confidence: 0.97, ModelVersionId: "v-1", ModelVersion: 1}
		for i := range 10 {
			job.Prediction.Scores = append(job.Prediction.Scores, &spotterv1.ClassScore{Label: string(rune('A' + i)), Score: 0.1})
		}
		data, _ := proto.Marshal(&job)
		if _, err := kv.Update(ctx, resp.GetJobId(), data, entry.Revision()); err != nil {
			t.Fatal(err)
		}
		got, err := e.client.GetInference(ctx, &spotterv1.GetInferenceRequest{JobId: resp.GetJobId()})
		if err != nil {
			t.Fatal(err)
		}
		p := got.GetJob().GetPrediction()
		if got.GetJob().GetStatus() != spotterv1.JobStatus_JOB_STATUS_SUCCEEDED || p.GetLabel() != "Forest" || len(p.GetScores()) != 10 || p.GetModelVersion() != 1 {
			t.Fatalf("job = %v", got.GetJob())
		}
	})
	t.Run("unknown job", func(t *testing.T) {
		spectest.Verifies(t, capability, "Job status can be read")
		_, err := e.client.GetInference(ctx, &spotterv1.GetInferenceRequest{JobId: uuid.NewString()})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("err = %v, want NOT_FOUND", err)
		}
	})
}
