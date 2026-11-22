package protoevents_test

import (
	"testing"

	spotterv1 "github.com/Pohi11/spotter/gen/go/spotter/v1"
	"github.com/Pohi11/spotter/services/registry/internal/adapters/protoevents"
	"github.com/Pohi11/spotter/services/registry/internal/domain"
)

// Every domain stage must have a protobuf value, or events would carry UNSPECIFIED.
func TestEveryStageHasAProtobufValue(t *testing.T) {
	for _, s := range domain.Stages() {
		if protoevents.Stage(s) == spotterv1.ModelVersionStage_MODEL_VERSION_STAGE_UNSPECIFIED {
			t.Errorf("stage %s maps to UNSPECIFIED", s)
		}
	}
}
