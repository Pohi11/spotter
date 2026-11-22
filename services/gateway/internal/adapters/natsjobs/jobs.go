// Package natsjobs keeps job status in the JetStream KV bucket and publishes job requests
// to the INFERENCE stream (decision record 0009).
package natsjobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/natsjs"
	"github.com/Pohi11/spotter/internal/natstrace"
	"github.com/Pohi11/spotter/services/gateway/internal/domain"
)

// Store implements ports.JobStore and ports.JobPublisher.
type Store struct {
	js jetstream.JetStream
	kv jetstream.KeyValue
}

// New binds to the jobs bucket, which natsjs.Ensure must have created.
func New(ctx context.Context, js jetstream.JetStream) (*Store, error) {
	kv, err := js.KeyValue(ctx, natsjs.JobsBucket)
	if err != nil {
		return nil, fmt.Errorf("bucket %s: %w", natsjs.JobsBucket, err)
	}
	return &Store{js: js, kv: kv}, nil
}

// Create stores a new pending job. The key is the job id.
func (s *Store) Create(ctx context.Context, job domain.Job) error {
	data, err := proto.Marshal(ToProto(job))
	if err != nil {
		return err
	}
	_, err = s.kv.Create(ctx, job.ID.String(), data)
	return err
}

// Get returns domain.ErrNotFound for an unknown (or expired) job.
func (s *Store) Get(ctx context.Context, id uuid.UUID) (domain.Job, error) {
	entry, err := s.kv.Get(ctx, id.String())
	if errors.Is(err, jetstream.ErrKeyNotFound) {
		return domain.Job{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Job{}, err
	}
	var pb spotterv1.InferenceJob
	if err := proto.Unmarshal(entry.Value(), &pb); err != nil {
		return domain.Job{}, fmt.Errorf("decode job %s: %w", id, err)
	}
	return FromProto(&pb)
}

// MarkFailed records a pending job as failed, using the entry's revision so a result a
// worker recorded in the meantime is never overwritten.
func (s *Store) MarkFailed(ctx context.Context, id uuid.UUID, reason string, at time.Time) error {
	entry, err := s.kv.Get(ctx, id.String())
	if err != nil {
		return err
	}
	var pb spotterv1.InferenceJob
	if err := proto.Unmarshal(entry.Value(), &pb); err != nil {
		return err
	}
	if pb.GetStatus() != spotterv1.JobStatus_JOB_STATUS_PENDING {
		return nil
	}
	pb.Status, pb.Error, pb.CompletedAt = spotterv1.JobStatus_JOB_STATUS_FAILED, reason, timestamppb.New(at)
	data, err := proto.Marshal(&pb)
	if err != nil {
		return err
	}
	_, err = s.kv.Update(ctx, id.String(), data, entry.Revision())
	return err
}

// PublishRequested publishes InferenceRequested with the job id as the message id, and
// waits for the stream's acknowledgement.
func (s *Store) PublishRequested(ctx context.Context, job domain.Job) error {
	data, err := proto.Marshal(&spotterv1.InferenceRequested{
		JobId: job.ID.String(), ModelName: job.ModelName, ImageUri: job.ImageURI, SubmittedAt: timestamppb.New(job.SubmittedAt),
	})
	if err != nil {
		return err
	}
	subject := natsjs.InferenceRequestedSubject(job.ModelName)
	ctx, span := tracer.Start(ctx, "publish "+subject, trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination.name", subject),
			attribute.String("spotter.job_id", job.ID.String()),
			attribute.String("spotter.model", job.ModelName),
		))
	defer span.End()

	msg := &nats.Msg{Subject: subject, Data: data, Header: nats.Header{}}
	msg.Header.Set(jetstream.MsgIDHeader, RequestedMsgID(job.ID))
	msg.Header.Set(natsjs.EventTypeHeader, "spotter.v1.InferenceRequested")
	natstrace.Inject(ctx, msg) // the worker continues this trace
	if _, err = s.js.PublishMsg(ctx, msg); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "publish failed")
	}
	return err
}

var tracer = otel.Tracer("github.com/Pohi11/spotter/services/gateway/internal/adapters/natsjobs")

var statusToProto = map[domain.Status]spotterv1.JobStatus{
	domain.StatusPending:   spotterv1.JobStatus_JOB_STATUS_PENDING,
	domain.StatusSucceeded: spotterv1.JobStatus_JOB_STATUS_SUCCEEDED,
	domain.StatusFailed:    spotterv1.JobStatus_JOB_STATUS_FAILED,
}

// ToProto converts a domain job to its stored and wire form.
func ToProto(j domain.Job) *spotterv1.InferenceJob {
	pb := &spotterv1.InferenceJob{
		JobId: j.ID.String(), ModelName: j.ModelName, Status: statusToProto[j.Status],
		ImageUri: j.ImageURI, SubmittedAt: timestamppb.New(j.SubmittedAt), Error: j.Error,
	}
	if !j.CompletedAt.IsZero() {
		pb.CompletedAt = timestamppb.New(j.CompletedAt)
	}
	if p := j.Prediction; p != nil {
		pb.Prediction = &spotterv1.Prediction{
			Label: p.Label, Confidence: p.Confidence, ModelVersionId: p.ModelVersionID, ModelVersion: int32(p.ModelVersion),
		}
		for _, s := range p.Scores {
			pb.Prediction.Scores = append(pb.Prediction.Scores, &spotterv1.ClassScore{Label: s.Label, Score: s.Score})
		}
	}
	return pb
}

// FromProto converts the stored form back to a domain job.
func FromProto(pb *spotterv1.InferenceJob) (domain.Job, error) {
	id, err := uuid.Parse(pb.GetJobId())
	if err != nil {
		return domain.Job{}, fmt.Errorf("stored job has a bad id %q: %w", pb.GetJobId(), err)
	}
	j := domain.Job{ID: id, ModelName: pb.GetModelName(), ImageURI: pb.GetImageUri(), Error: pb.GetError(),
		SubmittedAt: pb.GetSubmittedAt().AsTime()}
	for s, p := range statusToProto {
		if p == pb.GetStatus() {
			j.Status = s
		}
	}
	if pb.GetCompletedAt() != nil {
		j.CompletedAt = pb.GetCompletedAt().AsTime()
	}
	if p := pb.GetPrediction(); p != nil {
		pred := &domain.Prediction{Label: p.GetLabel(), Confidence: p.GetConfidence(),
			ModelVersionID: p.GetModelVersionId(), ModelVersion: int(p.GetModelVersion())}
		for _, s := range p.GetScores() {
			pred.Scores = append(pred.Scores, domain.ClassScore{Label: s.GetLabel(), Score: s.GetScore()})
		}
		j.Prediction = pred
	}
	return j, nil
}

// RequestedMsgID is the de-duplication id of a job's request. Requests, completions, and
// dead letters for one job share the INFERENCE stream and its duplicate window, so each
// kind of event gets its own prefix; otherwise the completion would be dropped as a
// duplicate of the request.
func RequestedMsgID(id uuid.UUID) string { return "requested-" + id.String() }
