// Package natsping sends a PingRequest over NATS request/reply and checks the reply.
//
// It is the smallest end-to-end proof that a Go program and a Python worker share the
// generated protobuf contract and can reach each other through the bus.
package natsping

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
)

// Subject is where ping responders listen.
const Subject = "spotter.v1.ping"

// ErrNoResponder means no service is subscribed to the ping subject.
var ErrNoResponder = errors.New("no ping responder is listening")

// Result is a decoded reply plus the measured round-trip time.
type Result struct {
	Reply     *spotterv1.PingReply
	RoundTrip time.Duration
}

// Requester is the one NATS capability Ping needs. *nats.Conn satisfies it.
type Requester interface {
	RequestWithContext(ctx context.Context, subject string, data []byte) (*nats.Msg, error)
}

// Ping sends one PingRequest and waits for the reply until ctx is done.
func Ping(ctx context.Context, nc Requester, sender string) (Result, error) {
	id, err := newID()
	if err != nil {
		return Result{}, err
	}
	req, err := proto.Marshal(&spotterv1.PingRequest{
		Id:     id,
		Sender: sender,
		SentAt: timestamppb.Now(),
	})
	if err != nil {
		return Result{}, fmt.Errorf("encode ping request: %w", err)
	}

	start := time.Now()
	msg, err := nc.RequestWithContext(ctx, Subject, req)
	rtt := time.Since(start)
	if errors.Is(err, nats.ErrNoResponders) {
		return Result{}, ErrNoResponder
	}
	if err != nil {
		return Result{}, fmt.Errorf("ping request: %w", err)
	}

	var reply spotterv1.PingReply
	if err := proto.Unmarshal(msg.Data, &reply); err != nil {
		return Result{}, fmt.Errorf("decode ping reply: %w", err)
	}
	if reply.GetId() != id {
		return Result{}, fmt.Errorf("ping reply id %q does not match request id %q", reply.GetId(), id)
	}
	return Result{Reply: &reply, RoundTrip: rtt}, nil
}

func newID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate ping id: %w", err)
	}
	return hex.EncodeToString(b), nil
}
