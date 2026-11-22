// Package natsjs declares Spotter's JetStream streams and key-value bucket in code
// (Everything as Code). Services call Ensure at startup; it is idempotent.
package natsjs

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// Stream, bucket, and subject names.
const (
	InferenceStream = "INFERENCE"
	RegistryStream  = "REGISTRY"
	JobsBucket      = "inference_jobs"

	// DuplicateWindow is how long a stream remembers message ids to drop duplicates.
	DuplicateWindow = 2 * time.Minute

	// EventTypeHeader carries the fully qualified protobuf message name of a payload.
	EventTypeHeader = "Spotter-Event-Type"
)

// InferenceRequestedSubject is where the gateway publishes jobs for a model.
func InferenceRequestedSubject(model string) string { return "inference.requested." + model }

// Streams returns the stream configurations, the single source of truth for them.
func Streams() []jetstream.StreamConfig {
	return []jetstream.StreamConfig{
		{
			Name:        InferenceStream,
			Description: "Inference jobs, completions, and dead letters",
			Subjects:    []string{"inference.>"},
			Storage:     jetstream.FileStorage,
			MaxAge:      24 * time.Hour,
			Duplicates:  DuplicateWindow,
		},
		{
			Name:        RegistryStream,
			Description: "Model registry events published from the registry's outbox",
			Subjects:    []string{"registry.events.>"},
			Storage:     jetstream.FileStorage,
			MaxAge:      7 * 24 * time.Hour,
			Duplicates:  DuplicateWindow,
		},
	}
}

// JobsBucketConfig is the key-value bucket holding each inference job's status.
func JobsBucketConfig() jetstream.KeyValueConfig {
	return jetstream.KeyValueConfig{
		Bucket:      JobsBucket,
		Description: "Inference job status by job id",
		TTL:         24 * time.Hour,
		History:     1,
		Storage:     jetstream.FileStorage,
	}
}

// Ensure creates or updates every stream and the jobs bucket.
func Ensure(ctx context.Context, js jetstream.JetStream) error {
	for _, cfg := range Streams() {
		if _, err := js.CreateOrUpdateStream(ctx, cfg); err != nil {
			return fmt.Errorf("stream %s: %w", cfg.Name, err)
		}
	}
	if _, err := js.CreateOrUpdateKeyValue(ctx, JobsBucketConfig()); err != nil {
		return fmt.Errorf("bucket %s: %w", JobsBucket, err)
	}
	return nil
}
