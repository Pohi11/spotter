// Package restapi is the gateway's REST front door: a grpc-gateway mux that turns
// HTTP/JSON requests into registry gRPC calls (paths come from registry.proto), plus the
// OpenAPI document.
package restapi

import (
	"context"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	// Registers google.rpc.BadRequest and the other error-detail types, so error details
	// sent by the registry can be rendered as JSON. Without it the gateway answers 500
	// "failed to marshal error message" for every validation error.
	_ "google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/gen/openapi"
)

// New returns a handler serving the registry's REST paths through conn, and
// GET /openapi.json. Errors use grpc-gateway's default handler: a JSON body with the
// gRPC code, message, and details, and the HTTP status from runtime.HTTPStatusFromCode.
func New(ctx context.Context, registry grpc.ClientConnInterface) (http.Handler, error) {
	mux := runtime.NewServeMux()
	if err := spotterv1.RegisterRegistryServiceHandlerClient(ctx, mux, spotterv1.NewRegistryServiceClient(registry)); err != nil {
		return nil, err
	}
	if err := mux.HandlePath(http.MethodGet, "/openapi.json", func(w http.ResponseWriter, _ *http.Request, _ map[string]string) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(openapi.Registry)
	}); err != nil {
		return nil, err
	}
	return mux, nil
}
