// Package adapters connects the monitor to JetStream, the registry, the object store, and
// OpenTelemetry metrics.
package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/internal/natsjs"
	"github.com/Pohi11/spotter/services/monitor/internal/app"
	"github.com/Pohi11/spotter/services/monitor/internal/domain"
	"github.com/Pohi11/spotter/services/monitor/internal/ports"
)

// ---- reference source: registry for the artifact URI, object store for reference.json ----

// S3Config locates the object store.
type S3Config struct{ Endpoint, AccessKey, SecretKey, Region string }

// References implements ports.ReferenceSource with a cache.
type References struct {
	registry spotterv1.RegistryServiceClient
	s3       *s3.Client
	mu       sync.Mutex
	cache    map[string]domain.Reference
}

func NewReferences(registry spotterv1.RegistryServiceClient, cfg S3Config) *References {
	return &References{registry: registry, cache: map[string]domain.Reference{}, s3: s3.New(s3.Options{
		BaseEndpoint: aws.String(cfg.Endpoint), Region: cfg.Region, UsePathStyle: true,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
	})}
}

func (r *References) Reference(ctx context.Context, versionID string) (domain.Reference, error) {
	r.mu.Lock()
	if ref, ok := r.cache[versionID]; ok {
		r.mu.Unlock()
		return ref, nil
	}
	r.mu.Unlock()

	resp, err := r.registry.GetVersion(ctx, &spotterv1.GetVersionRequest{VersionId: versionID})
	if status.Code(err) == codes.NotFound {
		return domain.Reference{}, ports.ErrNoReference
	}
	if err != nil {
		return domain.Reference{}, err
	}
	uri := resp.GetVersion().GetArtifactUri()
	bucket, key, _ := strings.Cut(strings.TrimPrefix(uri[:strings.LastIndex(uri, "/")], "s3://"), "/")
	out, err := r.s3.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key + "/reference.json")})
	var noKey *types.NoSuchKey
	if errors.As(err, &noKey) {
		return domain.Reference{}, ports.ErrNoReference
	}
	if err != nil {
		return domain.Reference{}, fmt.Errorf("get reference.json: %w", err)
	}
	defer out.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(out.Body); err != nil {
		return domain.Reference{}, err
	}
	var doc struct {
		Features map[string][]float64 `json:"features"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		return domain.Reference{}, fmt.Errorf("decode reference.json: %w", err)
	}
	ref := domain.Reference{VersionID: versionID, Values: map[domain.Feature][]float64{}}
	for _, f := range domain.Features {
		if len(doc.Features[string(f)]) == 0 {
			return domain.Reference{}, fmt.Errorf("reference.json lacks feature %s", f)
		}
		ref.Values[f] = doc.Features[string(f)]
	}
	r.mu.Lock()
	r.cache[versionID] = ref
	r.mu.Unlock()
	return ref, nil
}

// ---- publisher ----

// Publisher implements ports.Publisher on the MONITOR stream.
type Publisher struct{ JS jetstream.JetStream }

func featureDrift(results []domain.Result) []*spotterv1.FeatureDrift {
	features := make([]*spotterv1.FeatureDrift, len(results))
	for i, r := range results {
		features[i] = &spotterv1.FeatureDrift{Feature: string(r.Feature), Psi: r.PSI, KsStatistic: r.KSStatistic,
			KsPValue: r.KSPValue, ReferenceMean: r.ReferenceMean, WindowMean: r.WindowMean}
	}
	return features
}

// PublishEvaluation sends a DriftEvaluated on monitor.drift.evaluated.<model>.
func (p Publisher) PublishEvaluation(ctx context.Context, e ports.Evaluation) error {
	return p.send(ctx, "monitor.drift.evaluated."+e.Model, e.DedupKey, &spotterv1.DriftEvaluated{
		ModelName: e.Model, VersionId: e.VersionID, Version: int32(e.Version), Features: featureDrift(e.Results),
		WindowSize: int32(e.WindowSize), Drifting: e.Drifting, EvaluatedAt: timestamppb.New(e.At)})
}

func (p Publisher) Publish(ctx context.Context, e ports.DriftEvent) error {
	features := featureDrift(e.Results)
	var (
		subject string
		msg     proto.Message
	)
	switch e.Transition {
	case domain.Detected:
		subject = "monitor.drift.detected." + e.Model
		msg = &spotterv1.DriftDetected{ModelName: e.Model, VersionId: e.VersionID, Version: int32(e.Version),
			Features: features, WindowSize: int32(e.WindowSize), DetectedAt: timestamppb.New(e.At)}
	case domain.Resolved:
		subject = "monitor.drift.resolved." + e.Model
		msg = &spotterv1.DriftResolved{ModelName: e.Model, VersionId: e.VersionID, Version: int32(e.Version),
			Features: features, ResolvedAt: timestamppb.New(e.At)}
	default:
		return nil
	}
	return p.send(ctx, subject, e.DedupKey, msg)
}

func (p Publisher) send(ctx context.Context, subject, msgID string, msg proto.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	m := &nats.Msg{Subject: subject, Data: data, Header: nats.Header{}}
	m.Header.Set(jetstream.MsgIDHeader, msgID)
	m.Header.Set(natsjs.EventTypeHeader, string(msg.ProtoReflect().Descriptor().FullName()))
	_, err = p.JS.PublishMsg(ctx, m)
	return err
}

// ---- consumer ----

// ConsumerName is the monitor's durable consumer on the INFERENCE stream.
const ConsumerName = "drift-monitor"

// Consume feeds every InferenceCompleted event to the monitor until ctx is done. It acks
// after the monitor has processed an event and naks (to retry) when it failed.
func Consume(ctx context.Context, js jetstream.JetStream, mon *app.Monitor, logger *slog.Logger) error {
	cons, err := js.CreateOrUpdateConsumer(ctx, natsjs.InferenceStream, jetstream.ConsumerConfig{
		Durable: ConsumerName, FilterSubject: "inference.completed.>", AckPolicy: jetstream.AckExplicitPolicy,
	})
	if err != nil {
		return fmt.Errorf("consumer %s: %w", ConsumerName, err)
	}
	cc, err := cons.Consume(func(msg jetstream.Msg) {
		var ev spotterv1.InferenceCompleted
		if err := proto.Unmarshal(msg.Data(), &ev); err != nil || len(ev.GetInputStats().GetChannelMean()) != 3 {
			logger.Warn("skipping undecodable completion", "error", err)
			_ = msg.Term()
			return
		}
		meta, _ := msg.Metadata()
		var seq uint64
		if meta != nil {
			seq = meta.Sequence.Stream
		}
		stats := ev.GetInputStats()
		std := stats.GetChannelStd()
		obs := app.Observation{
			Model: ev.GetModelName(), VersionID: ev.GetPrediction().GetModelVersionId(), Version: int(ev.GetPrediction().GetModelVersion()),
			Sequence: seq,
			Sample: domain.Sample{float64(stats.GetChannelMean()[0]), float64(stats.GetChannelMean()[1]), float64(stats.GetChannelMean()[2]),
				(float64(std[0]) + float64(std[1]) + float64(std[2])) / 3, float64(ev.GetPrediction().GetConfidence())},
		}
		if err := mon.Observe(ctx, obs); err != nil {
			logger.Warn("observe failed; will retry", "error", err)
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	})
	if err != nil {
		return err
	}
	<-ctx.Done()
	cc.Drain()
	<-cc.Closed()
	return nil
}

// ---- metrics ----

// Metrics implements ports.Metrics with OpenTelemetry instruments (exported as Prometheus).
type Metrics struct {
	psi     metric.Float64Gauge
	skipped metric.Int64Counter
}

func NewMetrics() (*Metrics, error) {
	meter := otel.Meter("github.com/Pohi11/spotter/services/monitor")
	psi, err := meter.Float64Gauge("spotter_monitor_psi", metric.WithDescription("Latest PSI of a feature's window against the version's reference."))
	if err != nil {
		return nil, err
	}
	skipped, err := meter.Int64Counter("spotter_monitor_skipped_events", metric.WithDescription("Events skipped because their version has no reference."))
	if err != nil {
		return nil, err
	}
	return &Metrics{psi: psi, skipped: skipped}, nil
}

func (m *Metrics) PSI(model, versionID string, f domain.Feature, psi float64) {
	m.psi.Record(context.Background(), psi, metric.WithAttributes(
		attribute.String("model", model), attribute.String("version_id", versionID), attribute.String("feature", string(f))))
}

func (m *Metrics) Skipped(model, versionID string) {
	m.skipped.Add(context.Background(), 1, metric.WithAttributes(attribute.String("model", model), attribute.String("version_id", versionID)))
}
