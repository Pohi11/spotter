package main

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/services/registry/registryfake"
)

// TestBinaryRendersErrorDetails runs the gateway as its own compiled process. In-process
// tests share a binary with the fake registry, which links types (like
// google.rpc.BadRequest) the real gateway binary might not; this test caught exactly that.
func TestBinaryRendersErrorDetails(t *testing.T) {
	spectest.Verifies(t, capability, "Errors are returned as JSON with a matching HTTP status")
	if testing.Short() {
		t.Skip("builds and runs the gateway binary")
	}
	bin := filepath.Join(t.TempDir(), "gateway")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build gateway: %v\n%s", err, out)
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	reg := registryfake.Serve(lis)
	t.Cleanup(reg.GRPC.Stop)

	httpLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	httpAddr := httpLis.Addr().String()
	httpLis.Close()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, bin)
	vars, _ := baseEnv(t, lis.Addr().String())
	vars["HTTP_ADDR"] = httpAddr
	cmd.Env = cmd.Environ()
	for k, v := range vars {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); _ = cmd.Wait() })
	waitForLine(t, stdout, `"msg":"listening"`)

	base := "http://" + httpAddr
	post(t, base+"/v1/models", `{"name":"eurosat-classifier"}`)
	code, body := post(t, base+"/v1/models/eurosat-classifier/versions", `{"artifactUri":"s3://m/k","artifactSha256":"abc123","framework":"pytorch"}`)
	if code != http.StatusBadRequest || !strings.Contains(body, `"field":"artifact_sha256"`) {
		t.Fatalf("got %d %s, want 400 with a field violation for artifact_sha256", code, body)
	}
}

func post(t *testing.T, url, body string) (int, string) {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

func waitForLine(t *testing.T, r io.Reader, substr string) {
	t.Helper()
	found := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			if strings.Contains(sc.Text(), substr) {
				close(found)
				break
			}
		}
		_, _ = io.Copy(io.Discard, r)
	}()
	select {
	case <-found:
	case <-time.After(30 * time.Second):
		t.Fatalf("gateway never logged %q", substr)
	}
}
