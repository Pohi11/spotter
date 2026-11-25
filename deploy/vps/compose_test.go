// Package vps_test checks the public demo host (change phase-15-public-host). Rendering
// tests need Docker and skip without it; TestRunningHost checks a deployed host and runs
// only when SPOTTER_HOST_URL and SPOTTER_HOST_LOGIN (user:password) are set.
package vps_test

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Pohi11/spotter/internal/spectest"
)

const (
	capability = "platform-delivery"
	isolation  = "Only the proxy is reachable on the public demo host"
	access     = "The public demo host requires TLS and a password"
	caddyImage = "caddy:2.11.9-alpine"
)

var hostEnv = []string{
	"POSTGRES_PASSWORD=test-password-123", "S3_ACCESS_KEY=test-access", "S3_SECRET_KEY=test-secret-123",
	"SITE_ADDRESS=203-0-113-7.sslip.io", "BASIC_AUTH_USER=spotter",
	"BASIC_AUTH_HASH=$2a$14$Zkx19XLiW6VYouLHR5NmfOFU0z2GTNmpkT/5qqR7hx4IjWJPDhjvG",
}

func docker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not on PATH")
	}
}

func compose(t *testing.T, env []string, args ...string) ([]byte, error) {
	t.Helper()
	docker(t)
	cmd := exec.Command("docker", append([]string{"compose", "-f", "../demo/compose.yaml", "-f", "compose.yaml"}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	return cmd.CombinedOutput()
}

func TestRenderedForTheHost(t *testing.T) {
	spectest.Verifies(t, capability, isolation)
	out, err := compose(t, hostEnv, "--profile", "retrain", "config", "--format", "json")
	if err != nil {
		t.Fatalf("render: %v\n%s", err, out)
	}
	type port struct {
		Target    int    `json:"target"`
		Published string `json:"published"`
		Protocol  string `json:"protocol"`
	}
	var doc struct {
		Services map[string]struct {
			Ports   []port `json:"ports"`
			Volumes []struct {
				Source string `json:"source"`
				Target string `json:"target"`
			} `json:"volumes"`
		} `json:"services"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	for name, s := range doc.Services {
		if name != "caddy" && len(s.Ports) > 0 {
			t.Errorf("%s publishes %v; only caddy may", name, s.Ports)
		}
	}
	caddy, ok := doc.Services["caddy"]
	if !ok {
		t.Fatal("no caddy service")
	}
	var got []string
	for _, p := range caddy.Ports {
		got = append(got, p.Published+"/"+p.Protocol)
	}
	slices.Sort(got)
	if want := []string{"443/tcp", "443/udp", "80/tcp"}; !slices.Equal(got, want) {
		t.Errorf("caddy publishes %v, want %v", got, want)
	}
	var mounted bool
	for _, v := range caddy.Volumes {
		if v.Target == "/etc/caddy/Caddyfile" {
			abs, _ := filepath.Abs("Caddyfile")
			mounted = filepath.Clean(v.Source) == abs
		}
	}
	if !mounted {
		t.Errorf("Caddyfile from deploy/vps not mounted: %+v", caddy.Volumes)
	}
}

func TestMissingPasswordHashFails(t *testing.T) {
	spectest.Verifies(t, capability, isolation)
	env := append(slices.Clone(hostEnv), "BASIC_AUTH_HASH=")
	out, err := compose(t, env, "config")
	if err == nil || !strings.Contains(string(out), "BASIC_AUTH_HASH") {
		t.Fatalf("rendered without BASIC_AUTH_HASH (err %v):\n%.400s", err, out)
	}
}

// TestCaddyConfiguration adapts the Caddyfile with Caddy itself and checks that the site
// authenticates before anything else and then forwards to the dashboard.
func TestCaddyConfiguration(t *testing.T) {
	spectest.Verifies(t, capability, access)
	docker(t)
	abs, _ := filepath.Abs("Caddyfile")
	args := []string{"run", "--rm", "-v", abs + ":/etc/caddy/Caddyfile:ro"}
	for _, kv := range hostEnv[3:] {
		args = append(args, "-e", kv)
	}
	args = append(args, caddyImage, "caddy", "adapt", "--config", "/etc/caddy/Caddyfile")
	out, err := exec.Command("docker", args...).Output()
	if err != nil {
		t.Fatalf("caddy adapt: %v\n%s", err, out)
	}
	var cfg struct {
		Apps struct {
			HTTP struct {
				Servers map[string]struct {
					Listen []string `json:"listen"`
					Routes []struct {
						Match []struct {
							Host []string `json:"host"`
						} `json:"match"`
						Handle []json.RawMessage `json:"handle"`
					} `json:"routes"`
				} `json:"servers"`
			} `json:"http"`
		} `json:"apps"`
	}
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if len(cfg.Apps.HTTP.Servers) != 1 {
		t.Fatalf("servers %v", cfg.Apps.HTTP.Servers)
	}
	for _, srv := range cfg.Apps.HTTP.Servers {
		if !slices.Equal(srv.Listen, []string{":443"}) {
			t.Errorf("listens on %v, want :443 (Caddy redirects :80 to it)", srv.Listen)
		}
		if len(srv.Routes) != 1 || len(srv.Routes[0].Match) != 1 ||
			!slices.Equal(srv.Routes[0].Match[0].Host, []string{"203-0-113-7.sslip.io"}) {
			t.Fatalf("want one route for the site address, got %+v", srv.Routes)
		}
		// Every handler for the site sits in one subroute, in order.
		route := string(srv.Routes[0].Handle[0])
		auth := strings.Index(route, `"handler":"authentication"`)
		proxy := strings.Index(route, `"handler":"reverse_proxy"`)
		if auth < 0 || proxy < 0 || auth > proxy {
			t.Errorf("authentication at %d, reverse_proxy at %d; want authentication first\n%s", auth, proxy, route)
		}
		for _, want := range []string{`"http_basic"`, `"username":"spotter"`, `"dial":"dashboard:3000"`} {
			if !strings.Contains(route, want) {
				t.Errorf("route lacks %s\n%s", want, route)
			}
		}
	}
}

func TestRunningHost(t *testing.T) {
	spectest.Verifies(t, capability, access)
	site, login := os.Getenv("SPOTTER_HOST_URL"), os.Getenv("SPOTTER_HOST_LOGIN")
	if site == "" || login == "" {
		t.Skip("set SPOTTER_HOST_URL (https://...) and SPOTTER_HOST_LOGIN (user:password) to check a deployed host")
	}
	user, pass, _ := strings.Cut(login, ":")
	client := &http.Client{
		Timeout:       20 * time.Second,
		Transport:     &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	do := func(url string, auth bool) *http.Response {
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		if auth {
			req.SetBasicAuth(user, pass)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", url, err)
		}
		resp.Body.Close()
		return resp
	}
	if r := do(site+"/", false); r.StatusCode != http.StatusUnauthorized {
		t.Errorf("without credentials: %d, want 401", r.StatusCode)
	}
	if r := do(site+"/api/v1/models/eurosat-classifier", false); r.StatusCode != http.StatusUnauthorized {
		t.Errorf("API without credentials: %d, want 401", r.StatusCode)
	}
	if r := do(site+"/", true); r.StatusCode != http.StatusOK {
		t.Errorf("with credentials: %d, want 200", r.StatusCode)
	}
	plain := "http://" + strings.TrimPrefix(site, "https://") + "/"
	if r := do(plain, false); r.StatusCode != http.StatusPermanentRedirect ||
		!strings.HasPrefix(r.Header.Get("Location"), "https://") {
		t.Errorf("plain HTTP: %d to %q, want a permanent redirect to HTTPS", r.StatusCode, r.Header.Get("Location"))
	}
}
