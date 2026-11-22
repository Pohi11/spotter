package restapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/services/gateway/internal/restapi"
	"github.com/Pohi11/spotter/services/registry/registryfake"
)

const capability = "gateway-http"

// gateway serves the REST handler in front of a fake registry over an in-memory listener.
func gateway(t *testing.T) *httptest.Server {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	reg := registryfake.Serve(lis)
	t.Cleanup(reg.GRPC.Stop)
	conn, err := grpc.NewClient("passthrough:///registry",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return serve(t, conn)
}

func serve(t *testing.T, conn grpc.ClientConnInterface) *httptest.Server {
	t.Helper()
	h, err := restapi.New(context.Background(), conn, spotterv1.UnimplementedInferenceServiceServer{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

// call sends a request and decodes the JSON response into a map.
func call(t *testing.T, srv *httptest.Server, method, path, body string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("%s %s: response is not JSON: %s", method, path, raw)
		}
	}
	return resp.StatusCode, out
}

func mustOK(t *testing.T, code int, body map[string]any, what string) {
	t.Helper()
	if code != http.StatusOK {
		t.Fatalf("%s: status %d, body %v", what, code, body)
	}
}

const versionBody = `{"artifactUri":"s3://models/eurosat/v1.pt","artifactSha256":"` + registryfake.SHA + `","framework":"pytorch","metrics":{"accuracy":0.91}}`

func registerVersion(t *testing.T, srv *httptest.Server) map[string]any {
	t.Helper()
	code, body := call(t, srv, "POST", "/v1/models", `{"name":"eurosat-classifier"}`)
	if code != http.StatusOK && code != http.StatusConflict {
		t.Fatalf("register model: %d %v", code, body)
	}
	code, body = call(t, srv, "POST", "/v1/models/eurosat-classifier/versions", versionBody)
	mustOK(t, code, body, "register version")
	return body["version"].(map[string]any)
}

func TestEveryOperationIsRouted(t *testing.T) {
	spectest.Verifies(t, capability, "Registry operations are available over REST")
	srv := gateway(t)
	v := registerVersion(t, srv)
	id := v["id"].(string)

	code, body := call(t, srv, "GET", "/v1/models/eurosat-classifier", "")
	mustOK(t, code, body, "get model")
	code, body = call(t, srv, "GET", "/v1/models/eurosat-classifier/versions", "")
	mustOK(t, code, body, "list versions")
	if n := len(body["versions"].([]any)); n != 1 {
		t.Fatalf("listed %d versions", n)
	}
	code, body = call(t, srv, "GET", "/v1/versions/"+id, "")
	mustOK(t, code, body, "get version")
	code, body = call(t, srv, "POST", "/v1/versions/"+id+":transition",
		`{"toStage":"MODEL_VERSION_STAGE_VALIDATED","reason":"eval passed","expectedRevision":"1"}`)
	mustOK(t, code, body, "transition")
	code, body = call(t, srv, "GET", "/v1/versions/"+id+"/history", "")
	mustOK(t, code, body, "history")
	if n := len(body["changes"].([]any)); n != 1 {
		t.Fatalf("history has %d entries", n)
	}
}

func TestRegisterVersionOverREST(t *testing.T) {
	spectest.Verifies(t, capability, "Registry operations are available over REST")
	v := registerVersion(t, gateway(t))
	if v["version"] != float64(1) || v["stage"] != "MODEL_VERSION_STAGE_REGISTERED" || v["artifactSha256"] != registryfake.SHA {
		t.Fatalf("version = %v", v)
	}
}

func TestTransitionOverREST(t *testing.T) {
	spectest.Verifies(t, capability, "Registry operations are available over REST")
	srv := gateway(t)
	id := registerVersion(t, srv)["id"].(string)
	code, body := call(t, srv, "POST", "/v1/versions/"+id+":transition",
		`{"toStage":"MODEL_VERSION_STAGE_VALIDATED","reason":"eval passed","expectedRevision":"1"}`)
	mustOK(t, code, body, "transition")
	v := body["version"].(map[string]any)
	// int64 fields are JSON strings in protobuf's JSON mapping.
	if v["stage"] != "MODEL_VERSION_STAGE_VALIDATED" || v["revision"] != "2" {
		t.Fatalf("version = %v", v)
	}
}

func TestErrorsAreJSONWithMatchingStatus(t *testing.T) {
	srv := gateway(t)
	id := registerVersion(t, srv)["id"].(string)

	t.Run("unknown model", func(t *testing.T) {
		spectest.Verifies(t, capability, "Errors are returned as JSON with a matching HTTP status")
		code, body := call(t, srv, "GET", "/v1/models/no-such-model", "")
		if code != http.StatusNotFound || body["code"] != float64(5) {
			t.Fatalf("got %d %v, want 404 with code 5", code, body)
		}
	})
	t.Run("invalid field", func(t *testing.T) {
		spectest.Verifies(t, capability, "Errors are returned as JSON with a matching HTTP status")
		code, body := call(t, srv, "POST", "/v1/models/eurosat-classifier/versions",
			`{"artifactUri":"s3://m/k","artifactSha256":"abc123","framework":"pytorch"}`)
		raw, _ := json.Marshal(body["details"])
		if code != http.StatusBadRequest || !strings.Contains(string(raw), `"field":"artifact_sha256"`) {
			t.Fatalf("got %d %s, want 400 naming artifact_sha256", code, raw)
		}
	})
	t.Run("stale revision", func(t *testing.T) {
		spectest.Verifies(t, capability, "Errors are returned as JSON with a matching HTTP status")
		code, body := call(t, srv, "POST", "/v1/versions/"+id+":transition",
			`{"toStage":"MODEL_VERSION_STAGE_VALIDATED","reason":"late","expectedRevision":"9"}`)
		if code != http.StatusConflict {
			t.Fatalf("got %d %v, want 409", code, body)
		}
	})
}

func TestRegistryDownIs503(t *testing.T) {
	spectest.Verifies(t, capability, "Errors are returned as JSON with a matching HTTP status")
	// Nothing listens on this address once the listener is closed.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := lis.Addr().String()
	lis.Close()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	code, body := call(t, serve(t, conn), "GET", "/v1/models/eurosat-classifier", "")
	if code != http.StatusServiceUnavailable {
		t.Fatalf("got %d %v, want 503", code, body)
	}
}

func TestOpenAPIDocument(t *testing.T) {
	spectest.Verifies(t, capability, "The OpenAPI document is served")
	code, body := call(t, gateway(t), "GET", "/openapi.json", "")
	mustOK(t, code, body, "openapi")
	paths, _ := body["paths"].(map[string]any)
	for _, p := range []string{"/v1/models", "/v1/versions/{versionId}:transition"} {
		if _, ok := paths[p]; !ok {
			t.Errorf("OpenAPI paths missing %s (have %d paths)", p, len(paths))
		}
	}
}
