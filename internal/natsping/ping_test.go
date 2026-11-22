package natsping_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/natsping"
)

// startNATS runs a real NATS server inside the test process on a random port.
func startNATS(t *testing.T) *nats.Conn {
	t.Helper()
	opts := natstest.DefaultTestOptions
	opts.Port = server.RANDOM_PORT
	srv := natstest.RunServer(&opts)
	t.Cleanup(srv.Shutdown)

	nc, err := nats.Connect(srv.ClientURL())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(nc.Close)
	return nc
}

// respond subscribes a responder that builds its reply from the decoded request.
func respond(t *testing.T, nc *nats.Conn, build func(*spotterv1.PingRequest) *spotterv1.PingReply) {
	t.Helper()
	sub, err := nc.Subscribe(natsping.Subject, func(m *nats.Msg) {
		var req spotterv1.PingRequest
		if err := proto.Unmarshal(m.Data, &req); err != nil {
			t.Errorf("responder: decode: %v", err)
			return
		}
		out, err := proto.Marshal(build(&req))
		if err != nil {
			t.Errorf("responder: encode: %v", err)
			return
		}
		_ = m.Respond(out)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	t.Cleanup(func() { _ = sub.Unsubscribe() })
}

func TestPingReturnsTheMatchingReply(t *testing.T) {
	nc := startNATS(t)
	respond(t, nc, func(req *spotterv1.PingRequest) *spotterv1.PingReply {
		if req.GetSender() != "test" {
			t.Errorf("sender = %q, want %q", req.GetSender(), "test")
		}
		return &spotterv1.PingReply{Id: req.GetId(), Responder: "fake-worker", Runtime: "go"}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, err := natsping.Ping(ctx, nc, "test")
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if res.Reply.GetResponder() != "fake-worker" {
		t.Errorf("responder = %q, want fake-worker", res.Reply.GetResponder())
	}
	if res.RoundTrip <= 0 {
		t.Errorf("round trip = %v, want > 0", res.RoundTrip)
	}
}

func TestPingWithoutResponderFailsFast(t *testing.T) {
	nc := startNATS(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	start := time.Now()
	_, err := natsping.Ping(ctx, nc, "test")
	if !errors.Is(err, natsping.ErrNoResponder) {
		t.Fatalf("err = %v, want ErrNoResponder", err)
	}
	// NATS reports "no responders" immediately instead of waiting for the timeout.
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("took %v; expected an immediate no-responders error", elapsed)
	}
}

func TestPingRejectsAReplyForAnotherRequest(t *testing.T) {
	nc := startNATS(t)
	respond(t, nc, func(*spotterv1.PingRequest) *spotterv1.PingReply {
		return &spotterv1.PingReply{Id: "someone-else", Responder: "confused-worker"}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := natsping.Ping(ctx, nc, "test"); err == nil {
		t.Fatal("Ping accepted a reply with a different id")
	}
}

func TestPingStopsWhenTheContextExpires(t *testing.T) {
	nc := startNATS(t)
	// A responder that never answers.
	sub, err := nc.Subscribe(natsping.Subject, func(*nats.Msg) {})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = natsping.Ping(ctx, nc, "test")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
}
