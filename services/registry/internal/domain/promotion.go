package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrPromotionRequired: the generic transition refuses to enter production (decision
// record 0013); only PromoteVersion and RollbackModel do.
var ErrPromotionRequired = errors.New("production is entered only through PromoteVersion or RollbackModel")

// ErrGatesFailed: a promotion was refused because at least one gate failed.
var ErrGatesFailed = errors.New("promotion gates failed")

// ErrNothingToRollBack: no production version, or none previously retired from production.
var ErrNothingToRollBack = errors.New("nothing to roll back to")

// ShadowStats summarize a shadow version's comparisons with production.
type ShadowStats struct {
	Compared     int
	Agreed       int
	MeanShadowMS float64
}

// Approval is a person's recorded decision to allow a promotion.
type Approval struct {
	VersionID uuid.UUID
	Approver  string
	Reason    string
	At        time.Time
}

// NewApproval validates an approval for a version in stage.
func NewApproval(v ModelVersion, approver, reason string, at time.Time) (Approval, error) {
	if v.Stage != StageShadow {
		return Approval{}, &TransitionError{From: v.Stage, To: StageProduction}
	}
	if strings.TrimSpace(approver) == "" {
		return Approval{}, invalid("approver", "must not be blank")
	}
	if strings.TrimSpace(reason) == "" {
		return Approval{}, invalid("reason", "must not be blank")
	}
	return Approval{VersionID: v.ID, Approver: approver, Reason: reason, At: at}, nil
}

// GateConfig sets the promotion gates.
type GateConfig struct {
	AccuracyTolerance float64 // candidate accuracy ≥ production − tolerance
	MinComparisons    int
	MinAgreement      float64
	LatencyBudgetMS   float64
}

// DefaultGates: tolerance 0.01, ≥ 200 comparisons at ≥ 95% agreement, mean ≤ 25 ms.
var DefaultGates = GateConfig{AccuracyTolerance: 0.01, MinComparisons: 200, MinAgreement: 0.95, LatencyBudgetMS: 25}

// Gate is one promotion check and its outcome.
type Gate struct {
	Name   string
	Passed bool
	Detail string
}

// EvaluateGates checks a shadow candidate against production (nil if none). Pure.
func EvaluateGates(candidate ModelVersion, production *ModelVersion, stats ShadowStats, approvals int, cfg GateConfig) []Gate {
	var gates []Gate

	acc := Gate{Name: "accuracy"}
	cAcc, cOK := candidate.Metrics["accuracy"]
	switch {
	case production == nil:
		acc.Passed, acc.Detail = true, "no production version to compare with"
	case !cOK:
		acc.Detail = "candidate has no accuracy metric"
	default:
		pAcc := production.Metrics["accuracy"]
		acc.Passed = cAcc >= pAcc-cfg.AccuracyTolerance
		acc.Detail = fmt.Sprintf("holdout accuracy %.4f vs production %.4f (tolerance %.2f)", cAcc, pAcc, cfg.AccuracyTolerance)
	}
	gates = append(gates, acc)

	agree := Gate{Name: "shadow_agreement"}
	rate := 0.0
	if stats.Compared > 0 {
		rate = float64(stats.Agreed) / float64(stats.Compared)
	}
	agree.Passed = stats.Compared >= cfg.MinComparisons && rate >= cfg.MinAgreement
	agree.Detail = fmt.Sprintf("%d of at least %d comparisons, agreement %.1f%% (need %.0f%%)",
		stats.Compared, cfg.MinComparisons, 100*rate, 100*cfg.MinAgreement)
	gates = append(gates, agree)

	lat := Gate{Name: "shadow_latency"}
	lat.Passed = stats.Compared > 0 && stats.MeanShadowMS <= cfg.LatencyBudgetMS
	lat.Detail = fmt.Sprintf("mean shadow latency %.2f ms (budget %.0f ms)", stats.MeanShadowMS, cfg.LatencyBudgetMS)
	gates = append(gates, lat)

	gates = append(gates, Gate{Name: "approval", Passed: approvals > 0, Detail: fmt.Sprintf("%d approval(s) recorded", approvals)})
	return gates
}

// AllPassed reports whether every gate passed, and names the ones that didn't.
func AllPassed(gates []Gate) (bool, []string) {
	var failed []string
	for _, g := range gates {
		if !g.Passed {
			failed = append(failed, g.Name)
		}
	}
	return len(failed) == 0, failed
}
