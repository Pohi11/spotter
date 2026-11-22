// Package protoevents encodes registry domain events as the protobuf messages in
// proto/spotter/v1/registry_events.proto.
package protoevents

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/services/registry/internal/domain"
	"github.com/Pohi11/spotter/services/registry/internal/ports"
)

// Encoder implements ports.EventEncoder.
type Encoder struct{}

// Encode turns a domain event into an outbox record with a protobuf payload.
func (Encoder) Encode(e domain.Event) (ports.OutboxRecord, error) {
	var msg proto.Message
	switch ev := e.(type) {
	case domain.VersionRegistered:
		msg = &spotterv1.ModelVersionRegistered{
			EventId:        ev.ID.String(),
			ModelId:        ev.Version.ModelID.String(),
			ModelName:      ev.ModelName,
			VersionId:      ev.Version.ID.String(),
			Version:        int32(ev.Version.Number),
			ArtifactUri:    ev.Version.ArtifactURI,
			ArtifactSha256: ev.Version.ArtifactSHA256,
			Framework:      string(ev.Version.Framework),
			Metrics:        ev.Version.Metrics,
			OccurredAt:     timestamppb.New(ev.OccurredAt()),
		}
	case domain.StageChanged:
		msg = &spotterv1.ModelVersionStageChanged{
			EventId:    ev.ID.String(),
			ModelId:    ev.ModelID.String(),
			ModelName:  ev.ModelName,
			VersionId:  ev.Change.VersionID.String(),
			Version:    int32(ev.Number),
			FromStage:  Stage(ev.Change.From),
			ToStage:    Stage(ev.Change.To),
			Reason:     ev.Change.Reason,
			Revision:   ev.Change.Revision,
			OccurredAt: timestamppb.New(ev.OccurredAt()),
		}
	default:
		return ports.OutboxRecord{}, fmt.Errorf("protoevents: no encoding for %T", e)
	}

	payload, err := proto.Marshal(msg)
	if err != nil {
		return ports.OutboxRecord{}, fmt.Errorf("protoevents: encode %T: %w", e, err)
	}
	return ports.OutboxRecord{
		ID:          e.EventID(),
		Type:        string(msg.ProtoReflect().Descriptor().FullName()),
		AggregateID: e.AggregateID(),
		Payload:     payload,
		OccurredAt:  e.OccurredAt(),
	}, nil
}

var stages = map[domain.Stage]spotterv1.ModelVersionStage{
	domain.StageRegistered: spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_REGISTERED,
	domain.StageValidated:  spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_VALIDATED,
	domain.StageShadow:     spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_SHADOW,
	domain.StageProduction: spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_PRODUCTION,
	domain.StageRetired:    spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_RETIRED,
	domain.StageRejected:   spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_REJECTED,
	domain.StageRolledBack: spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_ROLLED_BACK,
}

// Stage maps a domain stage to its protobuf enum value.
func Stage(s domain.Stage) spotterv1.ModelVersionStage { return stages[s] }
