package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Pohi11/spotter/internal/chaos"
	"github.com/Pohi11/spotter/internal/spectest"
)

const capability = "failure-lab"

// The test binary doubles as a long-running "worker" when this variable is set.
func TestMain(m *testing.M) {
	if os.Getenv("SPOTTERCHAOS_FAKE_WORKER") == "1" {
		time.Sleep(time.Hour)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeWorker(t *testing.T) *Worker {
	t.Setenv("SPOTTERCHAOS_FAKE_WORKER", "1")
	w := &Worker{Args: []string{os.Args[0]}}
	t.Cleanup(w.Stop)
	return w
}

type recordingRunner struct{ calls []string }

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) error {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	return nil
}

var containers = map[string]string{"nats": "n", "postgres": "pg", "objectstore": "s3"}

func TestWorkerKillKillsAndStartsANewWorker(t *testing.T) {
	spectest.Verifies(t, capability, "A run injects one fault on a schedule and heals it")
	w := fakeWorker(t)
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	first := w.Pid()
	f, err := faultFor("worker-kill", w, &recordingRunner{}, containers)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Inject(context.Background()); err != nil {
		t.Fatal(err)
	}
	if w.Pid() != 0 {
		t.Fatal("worker still running after inject")
	}
	if err := f.Heal(context.Background()); err != nil {
		t.Fatal(err)
	}
	if p := w.Pid(); p == 0 || p == first {
		t.Fatalf("after heal pid %d (first %d); want a new process", p, first)
	}
}

func TestContainerFaultsRunTheirCommands(t *testing.T) {
	spectest.Verifies(t, capability, "A run injects one fault on a schedule and heals it")
	cases := map[string][]string{
		"postgres-pause":    {"docker pause pg", "docker unpause pg"},
		"objectstore-pause": {"docker pause s3", "docker unpause s3"},
		"nats-restart":      {"docker restart --time 0 n"},
		"none":              nil,
	}
	for name, want := range cases {
		r := &recordingRunner{}
		f, err := faultFor(name, nil, r, containers)
		if err != nil {
			t.Fatal(err)
		}
		_ = f.Inject(context.Background())
		_ = f.Heal(context.Background())
		if !slices.Equal(r.calls, want) {
			t.Errorf("%s ran %v, want %v", name, r.calls, want)
		}
	}
	if _, err := faultFor("disk-full", nil, &recordingRunner{}, containers); err == nil {
		t.Error("unknown fault accepted")
	}
}

func TestReportHoldsConfigurationAndTimeline(t *testing.T) {
	spectest.Verifies(t, capability, "Results are recorded with how they were measured")
	path := filepath.Join(t.TempDir(), "r.json")
	rep := chaos.Report{Requests: 3, Completed: 3, CompletedPerS: []int{1, 2}, RejectedPerS: []int{0, 0}}
	err := writeReport(path, Result{Date: "2026-11-22T00:00:00Z", Mode: "sync", Fault: "worker-kill",
		Config: ResultConfig{Concurrency: 4, DurationS: 40, FaultAtS: 10, HealAtS: 20}, Report: rep})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	b, _ := os.ReadFile(path)
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	cfg := got["config"].(map[string]any)
	r := got["report"].(map[string]any)
	if got["mode"] != "sync" || got["fault"] != "worker-kill" || cfg["fault_at_s"] != 10.0 || cfg["concurrency"] != 4.0 ||
		len(r["completed_per_second"].([]any)) != 2 {
		t.Fatalf("report %s", b)
	}
}
