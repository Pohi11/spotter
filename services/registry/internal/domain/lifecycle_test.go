package domain_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/services/registry/internal/domain"
)

const (
	lifecycle = "model-lifecycle"
	sha       = "6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b"
)

var t0 = time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

// The allowed transitions, written out independently of the implementation's table.
var allowed = map[[2]domain.Stage]bool{
	{domain.StageRegistered, domain.StageValidated}:  true,
	{domain.StageRegistered, domain.StageRejected}:   true,
	{domain.StageValidated, domain.StageShadow}:      true,
	{domain.StageValidated, domain.StageRejected}:    true,
	{domain.StageShadow, domain.StageProduction}:     true,
	{domain.StageShadow, domain.StageRejected}:       true,
	{domain.StageProduction, domain.StageRetired}:    true,
	{domain.StageProduction, domain.StageRolledBack}: true,
	{domain.StageRetired, domain.StageProduction}:    true,
}

func versionIn(t *testing.T, stage domain.Stage) domain.ModelVersion {
	t.Helper()
	v, err := domain.NewVersion(uuid.New(), uuid.New(), "s3://models/eurosat/v1.pt", sha, domain.FrameworkPyTorch, nil, t0)
	if err != nil {
		t.Fatal(err)
	}
	v.Stage = stage
	return v
}

// TestEveryStagePair checks all 49 (from, to) combinations against the spec's table.
func TestEveryStagePair(t *testing.T) {
	for _, from := range domain.Stages() {
		for _, to := range domain.Stages() {
			t.Run(fmt.Sprintf("%s_to_%s", from, to), func(t *testing.T) {
				if allowed[[2]domain.Stage{from, to}] {
					spectest.Verifies(t, lifecycle, "Only listed stage transitions are allowed")
				}
				v := versionIn(t, from)
				next, change, err := v.Transition(to, "test", t0.Add(time.Minute))

				if allowed[[2]domain.Stage{from, to}] {
					if err != nil {
						t.Fatalf("allowed transition failed: %v", err)
					}
					if next.Stage != to || next.Revision != v.Revision+1 || change.From != from || change.To != to {
						t.Fatalf("got stage %s rev %d change %+v", next.Stage, next.Revision, change)
					}
					return
				}

				spectest.Verifies(t, lifecycle, "Only listed stage transitions are allowed")
				if from == domain.StageRejected || from == domain.StageRolledBack {
					spectest.Verifies(t, lifecycle, "Rejected and rolled-back versions are final")
				}
				var te *domain.TransitionError
				if !errors.As(err, &te) || !errors.Is(err, domain.ErrIllegalTransition) {
					t.Fatalf("err = %v, want TransitionError", err)
				}
				if !strings.Contains(err.Error(), string(from)) || !strings.Contains(err.Error(), string(to)) {
					t.Errorf("error %q doesn't name both stages", err)
				}
				if next.Stage != from || next.Revision != v.Revision {
					t.Errorf("rejected transition changed the version: %+v", next)
				}
			})
		}
	}
}

func TestTableMatchesSpec(t *testing.T) {
	spectest.Verifies(t, lifecycle, "Only listed stage transitions are allowed")
	count := 0
	for _, from := range domain.Stages() {
		count += len(domain.AllowedFrom(from))
	}
	if count != len(allowed) {
		t.Fatalf("implementation allows %d transitions, spec lists %d", count, len(allowed))
	}
}

func TestSkippingValidationIsRejected(t *testing.T) {
	spectest.Verifies(t, lifecycle, "Only listed stage transitions are allowed")
	v := versionIn(t, domain.StageRegistered)
	if _, _, err := v.Transition(domain.StageProduction, "ship it", t0); !errors.Is(err, domain.ErrIllegalTransition) {
		t.Fatalf("err = %v, want illegal transition", err)
	}
}

func TestBlankReasonIsRejected(t *testing.T) {
	spectest.Verifies(t, lifecycle, "A transition requires a reason")
	v := versionIn(t, domain.StageRegistered)
	_, _, err := v.Transition(domain.StageValidated, "   ", t0)
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Field != "reason" {
		t.Fatalf("err = %v, want validation error on reason", err)
	}
}

func TestTransitionDoesNotShareMetricsWithTheOriginal(t *testing.T) {
	v, err := domain.NewVersion(uuid.New(), uuid.New(), "s3://m/k", sha, domain.FrameworkONNX, map[string]float64{"accuracy": 0.9}, t0)
	if err != nil {
		t.Fatal(err)
	}
	next, _, err := v.Transition(domain.StageValidated, "eval passed", t0)
	if err != nil {
		t.Fatal(err)
	}
	next.Metrics["accuracy"] = 0
	if v.Metrics["accuracy"] != 0.9 {
		t.Fatal("changing the transitioned copy changed the original")
	}
}
