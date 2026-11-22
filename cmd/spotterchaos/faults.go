package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/Pohi11/spotter/internal/chaos"
)

// Runner runs one command (Docker, in practice).
type Runner interface {
	Run(ctx context.Context, name string, args ...string) error
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) error {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w: %s", name, args, err, out)
	}
	return nil
}

// faultFor builds the named fault. containers maps "nats", "postgres", and "objectstore"
// to container names.
func faultFor(name string, worker *Worker, r Runner, containers map[string]string) (chaos.Fault, error) {
	switch name {
	case "none":
		return funcFault{}, nil
	case "worker-kill":
		// Killed without warning, like a crashed process; a new one starts at heal.
		return funcFault{inject: func(context.Context) error { return worker.Kill() }, heal: func(context.Context) error { return worker.Start() }}, nil
	case "nats-restart":
		c := containers["nats"]
		return funcFault{inject: func(ctx context.Context) error { return r.Run(ctx, "docker", "restart", "--time", "0", c) }}, nil
	case "postgres-pause", "objectstore-pause":
		c := containers[map[string]string{"postgres-pause": "postgres", "objectstore-pause": "objectstore"}[name]]
		return funcFault{
			inject: func(ctx context.Context) error { return r.Run(ctx, "docker", "pause", c) },
			heal:   func(ctx context.Context) error { return r.Run(ctx, "docker", "unpause", c) },
		}, nil
	}
	return nil, fmt.Errorf("unknown fault %q", name)
}

type funcFault struct{ inject, heal func(context.Context) error }

func (f funcFault) Inject(ctx context.Context) error {
	if f.inject == nil {
		return nil
	}
	return f.inject(ctx)
}

func (f funcFault) Heal(ctx context.Context) error {
	if f.heal == nil {
		return nil
	}
	return f.heal(ctx)
}

// Worker runs the inference worker as a child process so it can be killed without warning.
type Worker struct {
	Args    []string
	LogPath string

	mu  sync.Mutex
	cmd *exec.Cmd
}

func (w *Worker) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cmd != nil {
		return fmt.Errorf("worker already running (pid %d)", w.cmd.Process.Pid)
	}
	cmd := exec.Command(w.Args[0], w.Args[1:]...)
	var out io.Writer = io.Discard
	if w.LogPath != "" {
		f, err := os.OpenFile(w.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		out = f
	}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start worker: %w", err)
	}
	w.cmd = cmd
	return nil
}

// Kill ends the worker at once (SIGKILL on Unix, TerminateProcess on Windows).
func (w *Worker) Kill() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cmd == nil {
		return fmt.Errorf("no worker running")
	}
	err := w.cmd.Process.Kill()
	_ = w.cmd.Wait()
	w.cmd = nil
	return err
}

// Pid is the running worker's process id, or 0.
func (w *Worker) Pid() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cmd == nil {
		return 0
	}
	return w.cmd.Process.Pid
}

// Stop kills the worker if it runs; used at the end of a run.
func (w *Worker) Stop() {
	if w.Pid() != 0 {
		_ = w.Kill()
	}
}
