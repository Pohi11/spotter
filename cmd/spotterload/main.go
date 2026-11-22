// Command spotterload drives the gateway with concurrent inference requests and reports
// throughput and end-to-end latency, timing each job from just before submission to its
// InferenceCompleted event on NATS (so polling delay is not measured).
//
// Usage:
//
//	spotterload -images DIR [-gateway localhost:9091] [-nats-url URL] [-model eurosat-classifier]
//	            [-concurrency 8] [-duration 30s] [-timeout 30s] [-out report.json]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/loadgen"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "spotterload:", err)
		os.Exit(1)
	}
}

type submitter struct {
	client spotterv1.InferenceServiceClient
	model  string
}

func (s submitter) Submit(ctx context.Context, image []byte) (string, error) {
	resp, err := s.client.SubmitInference(ctx, &spotterv1.SubmitInferenceRequest{ModelName: s.model, Image: image})
	return resp.GetJobId(), err
}

func run() error {
	gateway := flag.String("gateway", "localhost:9091", "gateway gRPC address")
	natsURL := flag.String("nats-url", envOr("NATS_URL", nats.DefaultURL), "NATS server URL")
	model := flag.String("model", "eurosat-classifier", "model name")
	dir := flag.String("images", "", "directory of .jpg images to submit")
	concurrency := flag.Int("concurrency", 8, "concurrent clients")
	closed := flag.Bool("closed", true, "closed loop: each client waits for its result before the next submission")
	duration := flag.Duration("duration", 30*time.Second, "how long to submit")
	timeout := flag.Duration("timeout", 30*time.Second, "how long to wait for outstanding completions")
	out := flag.String("out", "", "write the report as JSON to this file")
	flag.Parse()

	paths, err := filepath.Glob(filepath.Join(*dir, "*.jpg"))
	if err != nil || len(paths) == 0 {
		return fmt.Errorf("no .jpg images in %q", *dir)
	}
	var images [][]byte
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		images = append(images, b)
	}

	nc, err := nats.Connect(*natsURL, nats.Name("spotterload"))
	if err != nil {
		return err
	}
	defer nc.Close()
	completions := make(chan loadgen.Completion, 100_000)
	sub, err := nc.Subscribe("inference.completed."+*model, func(m *nats.Msg) {
		at := time.Now()
		var ev spotterv1.InferenceCompleted
		if proto.Unmarshal(m.Data, &ev) == nil {
			completions <- loadgen.Completion{JobID: ev.GetJobId(), At: at}
		}
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()

	conn, err := grpc.NewClient(*gateway, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	fmt.Printf("submitting %d images from %d clients for %s...\n", len(images), *concurrency, *duration)
	rep := loadgen.Run(context.Background(), loadgen.Config{Concurrency: *concurrency, Duration: *duration, Timeout: *timeout, Images: images, ClosedLoop: *closed},
		submitter{client: spotterv1.NewInferenceServiceClient(conn), model: *model}, completions)

	fmt.Printf("submitted %d, failed %d, completed %d, lost %d in %.1fs\n", rep.Submitted, rep.FailedSubmissions, rep.Completed, rep.Lost, rep.Seconds)
	fmt.Printf("throughput %.1f jobs/s; latency p50 %.1f ms, p95 %.1f ms, p99 %.1f ms, max %.1f ms\n", rep.PerSecond, rep.P50ms, rep.P95ms, rep.P99ms, rep.MaxMs)
	if *out != "" {
		data, _ := json.MarshalIndent(map[string]any{
			"date": time.Now().UTC().Format(time.RFC3339), "model": *model, "concurrency": *concurrency,
			"duration_s": duration.Seconds(), "images": len(images), "closed_loop": *closed, "report": rep,
		}, "", "  ")
		return os.WriteFile(*out, data, 0o644)
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
