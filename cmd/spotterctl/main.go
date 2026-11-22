// Command spotterctl is the operator CLI for Spotter.
//
// Usage:
//
//	spotterctl ping  [-nats-url URL] [-timeout 2s]
//	spotterctl setup [-nats-url URL]
//	spotterctl infer [-gateway localhost:9091] [-model eurosat-classifier] [-timeout 30s] IMAGE...
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/natsjs"
	"github.com/Pohi11/spotter/internal/natsping"
)

const usage = "usage: spotterctl ping|setup [flags] | spotterctl infer [flags] IMAGE..."

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "spotterctl:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	switch args[0] {
	case "ping":
		return ping(args[1:])
	case "infer":
		return infer(args[1:])
	case "setup":
		return setup(args[1:])
	default:
		return errors.New(usage)
	}
}

func ping(args []string) error {
	fs := flag.NewFlagSet("ping", flag.ContinueOnError)
	natsURL := fs.String("nats-url", envOr("NATS_URL", nats.DefaultURL), "NATS server URL")
	timeout := fs.Duration("timeout", 2*time.Second, "how long to wait for a reply")
	if err := fs.Parse(args); err != nil {
		return err
	}
	nc, err := nats.Connect(*natsURL, nats.Name("spotterctl"))
	if err != nil {
		return fmt.Errorf("connect to %s: %w", *natsURL, err)
	}
	defer nc.Close()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	res, err := natsping.Ping(ctx, nc, "spotterctl")
	if err != nil {
		return err
	}
	fmt.Printf("reply from %s (%s) in %s\n", res.Reply.GetResponder(), res.Reply.GetRuntime(), res.RoundTrip.Round(time.Microsecond))
	return nil
}

// setup creates or updates the JetStream streams and bucket from their one definition
// (internal/natsjs). Services do this at startup; tools and CI use this command.
func setup(args []string) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	natsURL := fs.String("nats-url", envOr("NATS_URL", nats.DefaultURL), "NATS server URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	nc, err := nats.Connect(*natsURL, nats.Name("spotterctl"))
	if err != nil {
		return fmt.Errorf("connect to %s: %w", *natsURL, err)
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := natsjs.Ensure(ctx, js); err != nil {
		return err
	}
	fmt.Println("streams INFERENCE, REGISTRY and bucket inference_jobs are up to date")
	return nil
}

func infer(args []string) error {
	fs := flag.NewFlagSet("infer", flag.ContinueOnError)
	gateway := fs.String("gateway", envOr("GATEWAY_GRPC_ADDR", "localhost:9091"), "gateway gRPC address")
	model := fs.String("model", "eurosat-classifier", "model name")
	timeout := fs.Duration("timeout", 30*time.Second, "how long to wait for each result")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return errors.New("infer: give at least one image file")
	}
	conn, err := grpc.NewClient(*gateway, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()
	client := spotterv1.NewInferenceServiceClient(conn)

	for _, path := range fs.Args() {
		image, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		job, err := submitAndWait(ctx, client, *model, image)
		cancel()
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		switch job.GetStatus() {
		case spotterv1.JobStatus_JOB_STATUS_SUCCEEDED:
			p := job.GetPrediction()
			fmt.Printf("%s: %s (%.1f%%) by version %d in %s\n", path, p.GetLabel(), 100*p.GetConfidence(), p.GetModelVersion(), time.Since(start).Round(time.Millisecond))
		default:
			fmt.Printf("%s: failed: %s\n", path, job.GetError())
		}
	}
	return nil
}

// submitAndWait submits one image and polls until the job is no longer pending.
func submitAndWait(ctx context.Context, client spotterv1.InferenceServiceClient, model string, image []byte) (*spotterv1.InferenceJob, error) {
	resp, err := client.SubmitInference(ctx, &spotterv1.SubmitInferenceRequest{ModelName: model, Image: image})
	if err != nil {
		return nil, err
	}
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		got, err := client.GetInference(ctx, &spotterv1.GetInferenceRequest{JobId: resp.GetJobId()})
		if err != nil {
			return nil, err
		}
		if got.GetJob().GetStatus() != spotterv1.JobStatus_JOB_STATUS_PENDING {
			return got.GetJob(), nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("job %s still pending: %w", resp.GetJobId(), ctx.Err())
		case <-tick.C:
		}
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
