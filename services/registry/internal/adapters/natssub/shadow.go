// Package natssub feeds events from JetStream into the registry's use cases. It is a
// driving adapter, like grpcapi.
package natssub

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/natsjs"
	"github.com/Pohi11/spotter/services/registry/internal/app"
	"github.com/Pohi11/spotter/services/registry/internal/domain"
)

// ShadowConsumer is the registry's durable consumer for ShadowCompared events.
const ShadowConsumer = "registry-shadow-stats"

// ShadowSubjects is what the consumer reads: inference.shadow.<model>.
const ShadowSubjects = "inference.shadow.>"

// ConsumeShadow records every ShadowCompared event until ctx is done. A redelivered event
// is harmless: the repository stores one comparison per job. Events that can never be
// stored (undecodable, unknown version) are terminated rather than retried forever.
func ConsumeShadow(ctx context.Context, js jetstream.JetStream, svc *app.Service, logger *slog.Logger) error {
	cons, err := js.CreateOrUpdateConsumer(ctx, natsjs.InferenceStream, jetstream.ConsumerConfig{
		Durable: ShadowConsumer, FilterSubject: ShadowSubjects, AckPolicy: jetstream.AckExplicitPolicy,
	})
	if ctx.Err() != nil {
		return nil // stopped before the consumer was ready: nothing to drain
	}
	if err != nil {
		return fmt.Errorf("consumer %s: %w", ShadowConsumer, err)
	}
	cc, err := cons.Consume(func(msg jetstream.Msg) {
		c, err := decode(msg.Data())
		if err != nil {
			logger.Warn("dropping undecodable shadow comparison", "error", err)
			_ = msg.Term()
			return
		}
		err = svc.RecordShadowComparison(ctx, c)
		var ve *domain.ValidationError
		switch {
		case err == nil:
			_ = msg.Ack()
		case errors.Is(err, domain.ErrNotFound), errors.As(err, &ve):
			logger.Warn("dropping shadow comparison", "job_id", c.JobID, "error", err)
			_ = msg.Term()
		default:
			logger.Warn("recording shadow comparison failed; will retry", "job_id", c.JobID, "error", err)
			_ = msg.Nak()
		}
	})
	if err != nil {
		return err
	}
	<-ctx.Done()
	cc.Drain()
	<-cc.Closed()
	return nil
}

func decode(data []byte) (app.ShadowComparison, error) {
	var ev spotterv1.ShadowCompared
	if err := proto.Unmarshal(data, &ev); err != nil {
		return app.ShadowComparison{}, err
	}
	id, err := uuid.Parse(ev.GetShadowVersionId())
	if err != nil {
		return app.ShadowComparison{}, fmt.Errorf("shadow_version_id: %w", err)
	}
	return app.ShadowComparison{ShadowVersionID: id, JobID: ev.GetJobId(), Agree: ev.GetAgree(), ShadowMS: float64(ev.GetShadowMs())}, nil
}
