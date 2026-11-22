// Package natstest runs a real NATS server with JetStream inside the test process.
package natstest

import (
	"testing"

	"github.com/nats-io/nats-server/v2/server"
	natsserver "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Server is a running embedded NATS server.
type Server struct {
	*server.Server
	URL string
}

// Start runs a JetStream-enabled server on a random port, stored in a temp directory,
// and shuts it down when the test ends.
func Start(t testing.TB) *Server {
	t.Helper()
	opts := natsserver.DefaultTestOptions
	opts.Port = server.RANDOM_PORT
	opts.JetStream = true
	opts.StoreDir = t.TempDir()
	srv := natsserver.RunServer(&opts)
	t.Cleanup(srv.Shutdown)
	return &Server{Server: srv, URL: srv.ClientURL()}
}

// Connect returns a connection and a JetStream handle, closed when the test ends.
func (s *Server) Connect(t testing.TB) (*nats.Conn, jetstream.JetStream) {
	t.Helper()
	nc, err := nats.Connect(s.URL)
	if err != nil {
		t.Fatalf("natstest: connect: %v", err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatalf("natstest: jetstream: %v", err)
	}
	return nc, js
}
