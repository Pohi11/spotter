package natssub_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/natsjs"
	"github.com/Pohi11/spotter/internal/natstest"
	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/services/registry/internal/adapters/memory"
	"github.com/Pohi11/spotter/services/registry/internal/adapters/natssub"
	"github.com/Pohi11/spotter/services/registry/internal/adapters/protoevents"
	"github.com/Pohi11/spotter/services/registry/internal/app"
	"github.com/Pohi11/spotter/services/registry/internal/domain"
	"github.com/Pohi11/spotter/services/registry/internal/registrytest"
)

func publish(t *testing.T, js jetstream.JetStream, msgID string, data []byte) {
	t.Helper()
	m := &nats.Msg{Subject: "inference.shadow.eurosat-classifier", Data: data, Header: nats.Header{}}
	m.Header.Set(jetstream.MsgIDHeader, msgID)
	if _, err := js.PublishMsg(context.Background(), m); err != nil {
		t.Fatal(err)
	}
}

func TestShadowComparisonsFromJetStream(t *testing.T) {
	spectest.Verifies(t, "model-promotion", "Shadow comparisons are counted once per job")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, js := natstest.Start(t).Connect(t)
	if err := natsjs.Ensure(ctx, js); err != nil {
		t.Fatal(err)
	}
	repo := memory.New()
	svc := app.NewService(repo, protoevents.Encoder{}, registrytest.IDs{}, registrytest.NewClock())
	if _, err := svc.RegisterModel(ctx, "eurosat-classifier", ""); err != nil {
		t.Fatal(err)
	}
	v, err := svc.RegisterVersion(ctx, app.RegisterVersionInput{ModelName: "eurosat-classifier", ArtifactURI: "s3://m/v1/model.onnx",
		ArtifactSHA256: registrytest.SHA, Framework: domain.FrameworkONNX, Metrics: map[string]float64{"accuracy": 0.96}})
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- natssub.ConsumeShadow(ctx, js, svc, slog.New(slog.NewTextHandler(io.Discard, nil))) }()

	event := func(job string, agree bool, ms float32, version string) []byte {
		b, _ := proto.Marshal(&spotterv1.ShadowCompared{JobId: job, ModelName: "eurosat-classifier", ShadowVersionId: version, Agree: agree, ShadowMs: ms})
		return b
	}
	publish(t, js, "shadow-a", event("a", true, 2, v.ID.String()))
	publish(t, js, "shadow-b", event("b", false, 4, v.ID.String()))
	publish(t, js, "garbage", []byte{0xff, 0xff})                     // undecodable: dropped
	publish(t, js, "shadow-x", event("x", true, 1, uuid.NewString())) // unknown version: dropped
	publish(t, js, "shadow-c", event("c", true, 6, v.ID.String()))
	publish(t, js, "shadow-a-again", event("a", true, 2, v.ID.String())) // a redelivery with a new msg id

	deadline := time.Now().Add(5 * time.Second)
	for {
		s, err := repo.ShadowStats(ctx, v.ID)
		if err != nil {
			t.Fatal(err)
		}
		if s.Compared == 3 {
			if s.Agreed != 2 || s.MeanShadowMS != 4 {
				t.Fatalf("stats = %+v, want 2 agreed, mean 4 ms", s)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("stats = %+v after 5 s, want 3 comparisons", s)
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Every message, including the dropped ones, must be acked or terminated: nothing pending.
	cons, err := js.Consumer(ctx, natsjs.InferenceStream, natssub.ShadowConsumer)
	if err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for {
		info, err := cons.Info(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if info.NumAckPending == 0 && info.NumPending == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("consumer still has %d pending and %d unacked", info.NumPending, info.NumAckPending)
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
