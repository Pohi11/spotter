// Command spotterchaos measures the platform under one fault (spec: failure-lab; change
// phase-11-failure-lab). It starts the inference worker itself, drives closed-loop load
// through the gateway in asynchronous (submit, then a completion event) or synchronous
// (ClassifySync) mode, injects the fault, heals it, and writes a JSON report.
//
//	spotterchaos -mode async -fault worker-kill -images dir \
//	    -worker-cmd ".venv/Scripts/python.exe -m spotter_inference" -out report.json
//
// Faults: none, worker-kill, nats-restart, postgres-pause, objectstore-pause. The worker
// inherits this process's environment (set REGISTRY_ADDR, NATS_URL, S3_*, GRPC_PORT for
// sync mode, and SHADOW=off for comparable runs).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/chaos"
	"github.com/Pohi11/spotter/internal/natsjs"
	"github.com/Pohi11/spotter/internal/natsping"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "spotterchaos:", err)
		os.Exit(1)
	}
}

func run() error {
	mode := flag.String("mode", "async", "async (submit and wait for the completion event) or sync (ClassifySync)")
	faultName := flag.String("fault", "none", "none, worker-kill, nats-restart, postgres-pause, or objectstore-pause")
	gatewayAddr := flag.String("gateway", "localhost:9091", "gateway gRPC address")
	natsURL := flag.String("nats-url", envOr("NATS_URL", nats.DefaultURL), "NATS server URL")
	model := flag.String("model", "eurosat-classifier", "model name")
	dir := flag.String("images", "", "directory of .jpg images to send")
	workerCmd := flag.String("worker-cmd", "", "command that runs one inference worker (space-separated)")
	workerLog := flag.String("worker-log", "", "append the worker's output to this file")
	concurrency := flag.Int("concurrency", 4, "concurrent clients")
	duration := flag.Duration("duration", 40*time.Second, "how long clients send")
	faultAt := flag.Duration("fault-at", 10*time.Second, "when to inject the fault")
	healAt := flag.Duration("heal-at", 20*time.Second, "when to heal it")
	jobWait := flag.Duration("job-wait", 60*time.Second, "how long a client waits for its job before sending the next")
	drain := flag.Duration("drain", 60*time.Second, "how long to wait for outstanding jobs after the run")
	natsC := flag.String("nats-container", "spotter-nats-1", "NATS container")
	pgC := flag.String("postgres-container", "spotter-postgres-1", "Postgres container")
	s3C := flag.String("objectstore-container", "spotter-seaweedfs-1", "object store container")
	out := flag.String("out", "", "write the report as JSON to this file")
	flag.Parse()

	if *mode != "async" && *mode != "sync" {
		return fmt.Errorf("-mode must be async or sync")
	}
	images, err := readImages(*dir)
	if err != nil {
		return err
	}
	if *workerCmd == "" {
		return fmt.Errorf("-worker-cmd is required: the run starts and, for worker-kill, kills the worker")
	}

	nc, err := nats.Connect(*natsURL, nats.Name("spotterchaos"), nats.MaxReconnects(-1), nats.ReconnectWait(250*time.Millisecond))
	if err != nil {
		return err
	}
	defer nc.Close()

	worker := &Worker{Args: strings.Fields(*workerCmd), LogPath: *workerLog}
	if err := worker.Start(); err != nil {
		return err
	}
	defer worker.Stop()
	if err := waitForWorker(nc, 90*time.Second); err != nil {
		return err
	}
	time.Sleep(3 * time.Second) // the gRPC classifier starts after the consumer and the model

	fault, err := faultFor(*faultName, worker, execRunner{}, map[string]string{"nats": *natsC, "postgres": *pgC, "objectstore": *s3C})
	if err != nil {
		return err
	}

	conn, err := grpc.NewClient(*gatewayAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()
	inference := spotterv1.NewInferenceServiceClient(conn)

	completions := make(chan chaos.Completion, 100_000)
	start := time.Now()
	var client chaos.Client
	if *mode == "async" {
		client = asyncClient{c: inference, model: *model}
		stop, err := followCompletions(nc, *model, start, completions)
		if err != nil {
			return err
		}
		defer stop()
	} else {
		client = syncClient{c: inference, model: *model}
	}

	cfg := chaos.Config{Concurrency: *concurrency, Duration: *duration, FaultAt: *faultAt, HealAt: *healAt,
		JobWait: *jobWait, Drain: *drain, Images: images}
	fmt.Printf("%s mode, fault %s at %s, healed at %s, %d clients for %s\n", *mode, *faultName, *faultAt, *healAt, *concurrency, *duration)
	records, ev := chaos.Run(context.Background(), cfg, client, completions, fault)
	rep := chaos.Summarize(records, *faultAt, *healAt, *duration+*drain)
	recovery := "did not recover"
	if rep.RecoveryS != nil {
		recovery = fmt.Sprintf("recovered %.0f s after heal", *rep.RecoveryS)
	}
	fmt.Printf("requests %d: rejected %d, completed %d, lost %d, duplicates %d; p50 %.0f ms, p99 %.0f ms, max %.0f ms; %s\n",
		rep.Requests, rep.Rejected, rep.Completed, rep.Lost, rep.Duplicates, rep.P50ms, rep.P99ms, rep.MaxMs, recovery)
	if ev.InjectErr != "" || ev.HealErr != "" {
		fmt.Printf("fault errors: inject %q, heal %q\n", ev.InjectErr, ev.HealErr)
	}
	if *out == "" {
		return nil
	}
	return writeReport(*out, Result{
		Date: start.UTC().Format(time.RFC3339), Mode: *mode, Fault: *faultName, Model: *model,
		Config: ResultConfig{Concurrency: *concurrency, DurationS: duration.Seconds(), FaultAtS: faultAt.Seconds(),
			HealAtS: healAt.Seconds(), JobWaitS: jobWait.Seconds(), DrainS: drain.Seconds(), Images: len(images)},
		Machine: Machine{OS: runtime.GOOS, CPUs: runtime.NumCPU()},
		Events:  ResultEvents{InjectedAtS: ev.InjectedAt.Seconds(), HealedAtS: ev.HealedAt.Seconds(), InjectErr: ev.InjectErr, HealErr: ev.HealErr},
		Report:  rep,
	})
}

// Result is the JSON report (spec: failure-lab "Results are recorded with how they were
// measured").
type Result struct {
	Date    string       `json:"date"`
	Mode    string       `json:"mode"`
	Fault   string       `json:"fault"`
	Model   string       `json:"model"`
	Config  ResultConfig `json:"config"`
	Machine Machine      `json:"machine"`
	Events  ResultEvents `json:"events"`
	Report  chaos.Report `json:"report"`
}

type ResultConfig struct {
	Concurrency int     `json:"concurrency"`
	DurationS   float64 `json:"duration_s"`
	FaultAtS    float64 `json:"fault_at_s"`
	HealAtS     float64 `json:"heal_at_s"`
	JobWaitS    float64 `json:"job_wait_s"`
	DrainS      float64 `json:"drain_s"`
	Images      int     `json:"images"`
}

type Machine struct {
	OS   string `json:"os"`
	CPUs int    `json:"cpus"`
}

type ResultEvents struct {
	InjectedAtS float64 `json:"injected_at_s"`
	HealedAtS   float64 `json:"healed_at_s"`
	InjectErr   string  `json:"inject_error,omitempty"`
	HealErr     string  `json:"heal_error,omitempty"`
}

func writeReport(path string, r Result) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func readImages(dir string) ([][]byte, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.jpg"))
	if err != nil || len(paths) == 0 {
		return nil, fmt.Errorf("no .jpg images in %q", dir)
	}
	var images [][]byte
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		images = append(images, b)
	}
	return images, nil
}

func waitForWorker(nc *nats.Conn, limit time.Duration) error {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := natsping.Ping(ctx, nc, "spotterchaos")
		cancel()
		if err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("the worker didn't answer a NATS ping within %s", limit)
}

type asyncClient struct {
	c     spotterv1.InferenceServiceClient
	model string
}

func (a asyncClient) Send(ctx context.Context, image []byte) (string, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := a.c.SubmitInference(ctx, &spotterv1.SubmitInferenceRequest{ModelName: a.model, Image: image})
	return resp.GetJobId(), false, err
}

type syncClient struct {
	c     spotterv1.InferenceServiceClient
	model string
}

func (s syncClient) Send(ctx context.Context, image []byte) (string, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := s.c.ClassifySync(ctx, &spotterv1.ClassifySyncRequest{ModelName: s.model, Image: image})
	return "", err == nil, err
}

// followCompletions reads stored InferenceCompleted events from start on, with an ordered
// consumer: only stored messages reach the platform's consumers, so duplicates are counted
// as they would see them.
func followCompletions(nc *nats.Conn, model string, start time.Time, out chan<- chaos.Completion) (func(), error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	cons, err := js.OrderedConsumer(ctx, natsjs.InferenceStream, jetstream.OrderedConsumerConfig{
		FilterSubjects: []string{"inference.completed." + model},
		DeliverPolicy:  jetstream.DeliverByStartTimePolicy, OptStartTime: &start,
	})
	if err != nil {
		return nil, err
	}
	cc, err := cons.Consume(func(m jetstream.Msg) {
		var ev spotterv1.InferenceCompleted
		if proto.Unmarshal(m.Data(), &ev) == nil {
			out <- chaos.Completion{JobID: ev.GetJobId(), At: time.Now()}
		}
	})
	if err != nil {
		return nil, err
	}
	return cc.Stop, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
