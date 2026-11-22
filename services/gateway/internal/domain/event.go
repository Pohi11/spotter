package domain

import "time"

// Event is one message from the platform's event streams, as the event feed sends it.
type Event struct {
	Stream   string    `json:"stream"`
	Subject  string    `json:"subject"`
	Sequence uint64    `json:"sequence"`
	Time     time.Time `json:"time"`
	// Type is the fully qualified protobuf message name; empty when the publisher gave none.
	Type string `json:"type"`
	// Payload is the message as protobuf JSON, or nil when its type is unknown.
	Payload []byte `json:"-"`
}
