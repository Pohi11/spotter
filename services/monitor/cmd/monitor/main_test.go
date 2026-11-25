package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/natsjs"
	"github.com/Pohi11/spotter/internal/natstest"
	"github.com/Pohi11/spotter/internal/s3test"
	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/services/registry/registryfake"
)

const capability = "drift-monitoring"

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestMissingNATSURL(t *testing.T) {
	spectest.Verifies(t, capability, "The monitor process is configured from the environment and observable")
	err := run(context.Background(), env(nil), quiet, nil)
	if err == nil || !strings.Contains(err.Error(), "NATS_URL") {
		t.Fatalf("err = %v, want an error naming NATS_URL", err)
	}
}

// fixture: a registered version whose reference.json is in the object store.
func registeredVersionWithReference(t *testing.T, st s3test.Store, regAddr string) (versionID string) {
	t.Helper()
	ctx := context.Background()
	bucket := s3test.BucketName()
	client := s3.New(s3.Options{BaseEndpoint: aws.String(st.Endpoint), Region: st.Region, UsePathStyle: true,
		Credentials: credentials.NewStaticCredentialsProvider(st.AccessKey, st.SecretKey, "")})
	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatal(err)
	}
	r := rand.New(rand.NewPCG(7, 8))
	features := map[string][]float64{}
	for name, mean := range map[string]float64{"mean_r": 0.34, "mean_g": 0.38, "mean_b": 0.41, "contrast": 0.12, "confidence": 0.95} {
		v := make([]float64, 4000)
		for i := range v {
			v[i] = r.NormFloat64()*0.04 + mean
		}
		features[name] = v
	}
	body, _ := json.Marshal(map[string]any{"features": features})
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String("eurosat-classifier/abc/reference.json"), Body: bytes.NewReader(body)}); err != nil {
		t.Fatal(err)
	}
	conn, err := grpc.NewClient(regAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	reg := spotterv1.NewRegistryServiceClient(conn)
	if _, err := reg.RegisterModel(ctx, &spotterv1.RegisterModelRequest{Name: "eurosat-classifier"}); err != nil {
		t.Fatal(err)
	}
	v, err := reg.RegisterVersion(ctx, &spotterv1.RegisterVersionRequest{ModelName: "eurosat-classifier",
		ArtifactUri: "s3://" + bucket + "/eurosat-classifier/abc/model.pt", ArtifactSha256: registryfake.SHA, Framework: "onnx"})
	if err != nil {
		t.Fatal(err)
	}
	return v.GetVersion().GetId()
}

func TestDarkenedTrafficRaisesDriftDetectedAndPSIGauge(t *testing.T) {
	spectest.Verifies(t, capability, "Drift is announced once per change of state")
	spectest.Verifies(t, capability, "The monitor process is configured from the environment and observable")
	spectest.Verifies(t, capability, "Every evaluation is published")
	spectest.Verifies(t, capability, "Each comparison carries the bins behind its PSI")
	ns := natstest.Start(t)
	st := s3test.Start(t)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	reg := registryfake.Serve(lis)
	t.Cleanup(reg.GRPC.Stop)
	versionID := registeredVersionWithReference(t, st, lis.Addr().String())

	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	addr := ""
	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, env(map[string]string{"NATS_URL": ns.URL, "REGISTRY_ADDR": lis.Addr().String(),
			"S3_ENDPOINT": st.Endpoint, "S3_ACCESS_KEY": st.AccessKey, "S3_SECRET_KEY": st.SecretKey,
			"HTTP_ADDR": "127.0.0.1:0", "SHUTDOWN_TIMEOUT": "5s"}), quiet, func(_, a string) {
			mu.Lock()
			addr = a
			mu.Unlock()
			close(ready)
		})
	}()
	t.Cleanup(func() { cancel(); <-done })
	select {
	case <-ready:
	case err := <-done:
		t.Fatalf("monitor exited: %v", err)
	}

	// Darkened traffic: half the reference brightness.
	_, js := ns.Connect(t)
	r := rand.New(rand.NewPCG(9, 10))
	for i := range 250 {
		ev := &spotterv1.InferenceCompleted{JobId: "job", ModelName: "eurosat-classifier",
			Prediction: &spotterv1.Prediction{ModelVersionId: versionID, ModelVersion: 1, Confidence: float32(r.NormFloat64()*0.04 + 0.95)},
			InputStats: &spotterv1.InputStats{
				ChannelMean: []float32{float32((r.NormFloat64()*0.04 + 0.34) * 0.5), float32((r.NormFloat64()*0.04 + 0.38) * 0.5), float32((r.NormFloat64()*0.04 + 0.41) * 0.5)},
				ChannelStd:  []float32{0.12, 0.12, 0.12},
			}}
		data, _ := proto.Marshal(ev)
		msg := &nats.Msg{Subject: "inference.completed.eurosat-classifier", Data: data, Header: nats.Header{}}
		msg.Header.Set(jetstream.MsgIDHeader, "completed-"+string(rune('a'+i%26))+string(rune('a'+i/26)))
		if _, err := js.PublishMsg(context.Background(), msg); err != nil {
			t.Fatal(err)
		}
	}

	var detected spotterv1.DriftDetected
	deadline := time.Now().Add(20 * time.Second)
	for {
		s, err := js.Stream(context.Background(), natsjs.MonitorStream)
		if err == nil {
			if m, err := s.GetLastMsgForSubject(context.Background(), "monitor.drift.detected.eurosat-classifier"); err == nil {
				if err := proto.Unmarshal(m.Data, &detected); err != nil {
					t.Fatal(err)
				}
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("no DriftDetected within 20s")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if detected.GetVersionId() != versionID || len(detected.GetFeatures()) == 0 {
		t.Fatalf("DriftDetected = %v", &detected)
	}
	s, err := js.Stream(context.Background(), natsjs.MonitorStream)
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.GetLastMsgForSubject(context.Background(), "monitor.drift.evaluated.eurosat-classifier")
	if err != nil {
		t.Fatalf("no DriftEvaluated on the stream: %v", err)
	}
	var evaluated spotterv1.DriftEvaluated
	if err := proto.Unmarshal(m.Data, &evaluated); err != nil {
		t.Fatal(err)
	}
	if !evaluated.GetDrifting() || len(evaluated.GetFeatures()) != 5 || m.Header.Get(natsjs.EventTypeHeader) != "spotter.v1.DriftEvaluated" {
		t.Fatalf("DriftEvaluated = %v, header %q", &evaluated, m.Header.Get(natsjs.EventTypeHeader))
	}
	for _, f := range evaluated.GetFeatures() {
		if len(f.GetBinEdges()) != 9 || len(f.GetReferenceShares()) != 10 || len(f.GetWindowShares()) != 10 {
			t.Fatalf("%s published without its bins: %v", f.GetFeature(), f)
		}
	}

	mu.Lock()
	metricsURL := "http://" + addr + "/metrics"
	mu.Unlock()
	resp, err := http.Get(metricsURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	for _, f := range []string{"mean_r", "mean_g", "mean_b", "contrast", "confidence"} {
		if !strings.Contains(string(body), `feature="`+f+`"`) || !strings.Contains(string(body), "spotter_monitor_psi") {
			t.Fatalf("/metrics lacks spotter_monitor_psi for %s:\n%.1500s", f, body)
		}
	}
}
