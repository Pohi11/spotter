package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/services/registry/internal/domain"
)

const promotion = "model-promotion"

func withAccuracy(stage domain.Stage, acc float64) domain.ModelVersion {
	return domain.ModelVersion{ID: uuid.New(), Stage: stage, Metrics: map[string]float64{"accuracy": acc}}
}

func gate(gates []domain.Gate, name string) domain.Gate {
	for _, g := range gates {
		if g.Name == name {
			return g
		}
	}
	return domain.Gate{}
}

func TestAllGatesPass(t *testing.T) {
	spectest.Verifies(t, promotion, "Promotion gates are evaluated as code")
	prod := withAccuracy(domain.StageProduction, 0.9654)
	gates := domain.EvaluateGates(withAccuracy(domain.StageShadow, 0.9657), &prod,
		domain.ShadowStats{Compared: 500, Agreed: 497, MeanShadowMS: 3}, 1, domain.DefaultGates)
	if ok, failed := domain.AllPassed(gates); !ok {
		t.Fatalf("failed gates %v: %+v", failed, gates)
	}
}

func TestTooFewComparisons(t *testing.T) {
	spectest.Verifies(t, promotion, "Promotion gates are evaluated as code")
	prod := withAccuracy(domain.StageProduction, 0.96)
	g := gate(domain.EvaluateGates(withAccuracy(domain.StageShadow, 0.97), &prod,
		domain.ShadowStats{Compared: 120, Agreed: 120, MeanShadowMS: 3}, 1, domain.DefaultGates), "shadow_agreement")
	if g.Passed || !strings.Contains(g.Detail, "120 of at least 200") {
		t.Fatalf("gate = %+v", g)
	}
}

func TestNoApproval(t *testing.T) {
	spectest.Verifies(t, promotion, "Promotion gates are evaluated as code")
	gates := domain.EvaluateGates(withAccuracy(domain.StageShadow, 0.97), nil,
		domain.ShadowStats{Compared: 500, Agreed: 500, MeanShadowMS: 3}, 0, domain.DefaultGates)
	ok, failed := domain.AllPassed(gates)
	if ok || len(failed) != 1 || failed[0] != "approval" {
		t.Fatalf("failed = %v, want only approval", failed)
	}
}

func TestFirstVersionNeedsAnApprovalButNoShadowEvidence(t *testing.T) {
	spectest.Verifies(t, promotion, "A model's first version is promoted on approval")
	none := domain.ShadowStats{}
	gates := domain.EvaluateGates(withAccuracy(domain.StageShadow, 0.97), nil, none, 0, domain.DefaultGates)
	if ok, failed := domain.AllPassed(gates); ok || len(failed) != 1 || failed[0] != "approval" {
		t.Fatalf("without approval: failed = %v, want only approval", failed)
	}
	gates = domain.EvaluateGates(withAccuracy(domain.StageShadow, 0.97), nil, none, 1, domain.DefaultGates)
	if ok, failed := domain.AllPassed(gates); !ok {
		t.Fatalf("with approval: failed = %v", failed)
	}
	for _, name := range []string{"shadow_agreement", "shadow_latency"} {
		if g := gate(gates, name); !strings.Contains(g.Detail, "no production version") {
			t.Errorf("%s detail %q does not say why it passed", name, g.Detail)
		}
	}
}

func TestAccuracyBelowTolerance(t *testing.T) {
	prod := withAccuracy(domain.StageProduction, 0.9657)
	g := gate(domain.EvaluateGates(withAccuracy(domain.StageShadow, 0.94), &prod,
		domain.ShadowStats{Compared: 500, Agreed: 500, MeanShadowMS: 3}, 1, domain.DefaultGates), "accuracy")
	if g.Passed {
		t.Fatalf("accuracy gate passed at 0.94 vs 0.9657: %+v", g)
	}
}

func TestApprovals(t *testing.T) {
	spectest.Verifies(t, promotion, "Approvals are recorded")
	if _, err := domain.NewApproval(withAccuracy(domain.StageShadow, 0.97), "ana", "agreement 99.4% over 500 jobs", t0); err != nil {
		t.Fatalf("approval of a shadow version: %v", err)
	}
	if _, err := domain.NewApproval(withAccuracy(domain.StageValidated, 0.97), "ana", "x", t0); !errors.Is(err, domain.ErrIllegalTransition) {
		t.Fatalf("approval of a validated version: err = %v", err)
	}
	var ve *domain.ValidationError
	if _, err := domain.NewApproval(withAccuracy(domain.StageShadow, 0.97), " ", "x", t0); !errors.As(err, &ve) || ve.Field != "approver" {
		t.Fatalf("blank approver: err = %v", err)
	}
}
