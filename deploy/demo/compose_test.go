// Package demo_test checks the demo deployment and the image workflow (change
// phase-12-delivery). Rendering tests need Docker and skip without it; TestRunningDemo
// starts the whole demo and runs only when SPOTTER_DEMO_TEST=1 (it uses images tagged
// local/spotter-*:dev).
package demo_test

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/Pohi11/spotter/internal/spectest"
)

const capability = "platform-delivery"

var secrets = []string{"POSTGRES_PASSWORD=test-password-123", "S3_ACCESS_KEY=test-access", "S3_SECRET_KEY=test-secret-123"}

func compose(t *testing.T, env []string, args ...string) ([]byte, error) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not on PATH")
	}
	cmd := exec.Command("docker", append([]string{"compose", "-f", "compose.yaml"}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	return cmd.CombinedOutput()
}

func TestRenderedWithSecrets(t *testing.T) {
	spectest.Verifies(t, capability, "The demo runs the platform on one machine")
	out, err := compose(t, secrets, "config", "--format", "json")
	if err != nil {
		t.Fatalf("render: %v\n%s", err, out)
	}
	var doc struct {
		Services map[string]struct {
			Image string `json:"image"`
			Ports []any  `json:"ports"`
		} `json:"services"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	var names []string
	for name, s := range doc.Services {
		names = append(names, name)
		if (len(s.Ports) > 0) != (name == "dashboard") {
			t.Errorf("%s publishes ports %v; only the dashboard may", name, s.Ports)
		}
	}
	slices.Sort(names)
	want := []string{"dashboard", "gateway", "monitor", "nats", "postgres", "registry", "seaweedfs", "worker"}
	if !slices.Equal(names, want) {
		t.Fatalf("services %v, want %v", names, want)
	}
	if img := doc.Services["gateway"].Image; img != "ghcr.io/pohi11/spotter-go:main" {
		t.Errorf("default gateway image %q", img)
	}
}

func TestRetrainProfileAddsTheWatcher(t *testing.T) {
	spectest.Verifies(t, capability, "The demo can retrain on drift without a terminal")
	type service struct {
		Command []string `json:"command"`
		Ports   []any    `json:"ports"`
		Volumes []struct {
			Source string `json:"source"`
			Target string `json:"target"`
		} `json:"volumes"`
	}
	render := func(args ...string) map[string]service {
		out, err := compose(t, secrets, append(args, "config", "--format", "json")...)
		if err != nil {
			t.Fatalf("render %v: %v\n%s", args, err, out)
		}
		var doc struct{ Services map[string]service }
		if err := json.Unmarshal(out, &doc); err != nil {
			t.Fatal(err)
		}
		return doc.Services
	}
	if _, ok := render()["retrainer"]; ok {
		t.Fatal("retrainer rendered without the retrain profile")
	}
	services := render("--profile", "retrain", "--profile", "bootstrap")
	r, ok := services["retrainer"]
	if !ok {
		t.Fatal("no retrainer with the retrain profile")
	}
	if !slices.Equal(r.Command, []string{"spotter-retrain", "--watch"}) || len(r.Ports) > 0 {
		t.Fatalf("retrainer command %v, ports %v", r.Command, r.Ports)
	}
	b := services["bootstrap"]
	if len(r.Volumes) != 1 || len(b.Volumes) != 1 || r.Volumes[0] != b.Volumes[0] {
		t.Fatalf("retrainer volumes %v, bootstrap volumes %v; want the same dataset volume", r.Volumes, b.Volumes)
	}
}

func TestMissingSecretFails(t *testing.T) {
	spectest.Verifies(t, capability, "The demo runs the platform on one machine")
	env := []string{"POSTGRES_PASSWORD=", "S3_ACCESS_KEY=a", "S3_SECRET_KEY=b"}
	out, err := compose(t, env, "config")
	if err == nil || !strings.Contains(string(out), "POSTGRES_PASSWORD") {
		t.Fatalf("rendered without POSTGRES_PASSWORD (err %v):\n%.400s", err, out)
	}
}

func TestImageWorkflow(t *testing.T) {
	spectest.Verifies(t, capability, "Images are built on every change and published from main")
	b, err := os.ReadFile("../../.github/workflows/images.yml")
	if err != nil {
		t.Fatal(err)
	}
	var wf struct {
		On   map[string]any `yaml:"on"`
		Jobs map[string]struct {
			Strategy struct {
				Matrix struct {
					Include []struct{ Image string } `yaml:"include"`
				} `yaml:"matrix"`
			} `yaml:"strategy"`
			Steps []struct {
				Uses string            `yaml:"uses"`
				If   string            `yaml:"if"`
				With map[string]string `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(b, &wf); err != nil {
		t.Fatal(err)
	}
	if _, ok := wf.On["pull_request"]; !ok {
		t.Error("not built on pull requests")
	}
	job := wf.Jobs["build"]
	var images []string
	for _, m := range job.Strategy.Matrix.Include {
		images = append(images, m.Image)
	}
	slices.Sort(images)
	if !slices.Equal(images, []string{"spotter-dashboard", "spotter-go", "spotter-python"}) {
		t.Errorf("images %v", images)
	}
	var sawLogin, sawBuild bool
	for _, s := range job.Steps {
		switch {
		case strings.HasPrefix(s.Uses, "docker/login-action"):
			sawLogin = true
			if s.If != "github.event_name == 'push'" || s.With["registry"] != "ghcr.io" {
				t.Errorf("login step %+v: want ghcr.io, push events only", s)
			}
		case strings.HasPrefix(s.Uses, "docker/build-push-action"):
			sawBuild = true
			if s.With["platforms"] != "linux/amd64,linux/arm64" || s.With["push"] != "${{ github.event_name == 'push' }}" {
				t.Errorf("build step %+v", s.With)
			}
		case strings.HasPrefix(s.Uses, "docker/metadata-action"):
			if !strings.Contains(s.With["tags"], "type=sha,format=long") || !strings.Contains(s.With["tags"], "value=main") {
				t.Errorf("tags %q", s.With["tags"])
			}
		}
	}
	if !sawLogin || !sawBuild {
		t.Error("login or build step missing")
	}
}

func TestRunningDemo(t *testing.T) {
	spectest.Verifies(t, capability, "The demo runs the platform on one machine")
	if os.Getenv("SPOTTER_DEMO_TEST") != "1" {
		t.Skip("set SPOTTER_DEMO_TEST=1 to start the demo (needs local/spotter-*:dev images)")
	}
	env := append([]string{"IMAGE_PREFIX=local", "IMAGE_TAG=dev", "DASHBOARD_PORT=8300", "COMPOSE_PROJECT_NAME=spotter-demo-test"}, secrets...)
	t.Cleanup(func() { _, _ = compose(t, env, "down", "-v") })
	if out, err := compose(t, env, "up", "-d", "--wait", "--wait-timeout", "300"); err != nil {
		t.Fatalf("up: %v\n%s", err, out)
	}
	get := func(path string) int {
		resp, err := http.Get("http://localhost:8300" + path)
		if err != nil {
			return 0
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	deadline := time.Now().Add(2 * time.Minute)
	// The dashboard, then its proxy through the gateway to the registry: no model yet, so
	// 404 from the registry (not the proxy's own 404 for an unlisted route) proves the chain.
	for get("/") != http.StatusOK || get("/api/v1/models/eurosat-classifier") != http.StatusNotFound {
		if time.Now().After(deadline) {
			out, _ := compose(t, env, "ps")
			t.Fatalf("dashboard %d, model lookup %d after 2 min\n%s", get("/"), get("/api/v1/models/eurosat-classifier"), out)
		}
		time.Sleep(2 * time.Second)
	}
	out, err := compose(t, env, "ps", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		var s struct{ Service, State, Health string }
		if json.Unmarshal([]byte(line), &s) == nil && (s.State != "running" || s.Health == "unhealthy") {
			t.Errorf("%s: state %s health %s", s.Service, s.State, s.Health)
		}
	}
}
