package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/Pohi11/spotter/services/registry/internal/domain"
	"github.com/Pohi11/spotter/services/registry/internal/ports"
)

// Gates is the gate configuration the promotion use cases apply.
var Gates = domain.DefaultGates

// ApproveVersion records a person's approval of a shadow version.
func (s *Service) ApproveVersion(ctx context.Context, versionID uuid.UUID, approver, reason string) (domain.Approval, error) {
	v, err := s.repo.Version(ctx, versionID)
	if err != nil {
		return domain.Approval{}, err
	}
	a, err := domain.NewApproval(v, approver, reason, s.clock.Now())
	if err != nil {
		return domain.Approval{}, err
	}
	return a, s.repo.AddApproval(ctx, a)
}

// PromotionGates evaluates the gates for a version without changing anything.
func (s *Service) PromotionGates(ctx context.Context, versionID uuid.UUID) ([]domain.Gate, error) {
	gates, _, _, err := s.gates(ctx, versionID)
	return gates, err
}

func (s *Service) gates(ctx context.Context, versionID uuid.UUID) ([]domain.Gate, domain.ModelVersion, *domain.ModelVersion, error) {
	v, err := s.repo.Version(ctx, versionID)
	if err != nil {
		return nil, domain.ModelVersion{}, nil, err
	}
	var prod *domain.ModelVersion
	if p, err := s.repo.ProductionVersion(ctx, v.ModelID); err == nil {
		prod = &p
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, domain.ModelVersion{}, nil, err
	}
	stats, err := s.repo.ShadowStats(ctx, v.ID)
	if err != nil {
		return nil, domain.ModelVersion{}, nil, err
	}
	approvals, err := s.repo.Approvals(ctx, v.ID)
	if err != nil {
		return nil, domain.ModelVersion{}, nil, err
	}
	return domain.EvaluateGates(v, prod, stats, approvals, Gates), v, prod, nil
}

// PromoteVersion promotes a shadow version whose gates all pass, retiring the current
// production version in the same transaction. It returns the promoted and retired versions.
func (s *Service) PromoteVersion(ctx context.Context, versionID uuid.UUID, reason string) (domain.ModelVersion, *domain.ModelVersion, error) {
	gates, v, prod, err := s.gates(ctx, versionID)
	if err != nil {
		return domain.ModelVersion{}, nil, err
	}
	if v.Stage != domain.StageShadow {
		return domain.ModelVersion{}, nil, &domain.TransitionError{From: v.Stage, To: domain.StageProduction}
	}
	if ok, failed := domain.AllPassed(gates); !ok {
		return domain.ModelVersion{}, nil, fmt.Errorf("%w: %s", domain.ErrGatesFailed, strings.Join(failed, ", "))
	}
	if strings.TrimSpace(reason) == "" {
		reason = "promoted: all gates passed"
	}
	in, inChange, err := v.Transition(domain.StageProduction, reason, s.clock.Now())
	if err != nil {
		return domain.ModelVersion{}, nil, err
	}
	var out *domain.ModelVersion
	var outExpected int64
	changes := []domain.StageChange{}
	if prod != nil {
		o, oc, err := prod.Transition(domain.StageRetired, fmt.Sprintf("replaced by version %d", v.Number), s.clock.Now())
		if err != nil {
			return domain.ModelVersion{}, nil, err
		}
		out, outExpected = &o, prod.Revision
		changes = append(changes, oc)
	}
	changes = append(changes, inChange)
	if err := s.swap(ctx, out, outExpected, in, v.Revision, changes); err != nil {
		return domain.ModelVersion{}, nil, fmt.Errorf("promote version %s: %w", versionID, err)
	}
	return in, out, nil
}

// RollbackModel marks the production version rolled back and restores the version most
// recently retired from production, in one transaction.
func (s *Service) RollbackModel(ctx context.Context, modelName, reason string) (restored, rolledBack domain.ModelVersion, err error) {
	m, err := s.repo.ModelByName(ctx, modelName)
	if err != nil {
		return restored, rolledBack, err
	}
	prod, err := s.repo.ProductionVersion(ctx, m.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return restored, rolledBack, fmt.Errorf("%w: model %q has no production version", domain.ErrNothingToRollBack, modelName)
	}
	if err != nil {
		return restored, rolledBack, err
	}
	prev, err := s.repo.LastRetiredFromProduction(ctx, m.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return restored, rolledBack, fmt.Errorf("%w: no version was retired from production", domain.ErrNothingToRollBack)
	}
	if err != nil {
		return restored, rolledBack, err
	}
	if strings.TrimSpace(reason) == "" {
		reason = "rollback"
	}
	out, outChange, err := prod.Transition(domain.StageRolledBack, reason, s.clock.Now())
	if err != nil {
		return restored, rolledBack, err
	}
	in, inChange, err := prev.Transition(domain.StageProduction, fmt.Sprintf("restored by rollback of version %d: %s", prod.Number, reason), s.clock.Now())
	if err != nil {
		return restored, rolledBack, err
	}
	if err := s.swap(ctx, &out, prod.Revision, in, prev.Revision, []domain.StageChange{outChange, inChange}); err != nil {
		return restored, rolledBack, fmt.Errorf("roll back %q: %w", modelName, err)
	}
	return in, out, nil
}

// swap encodes a StageChanged event for each change and stores the swap atomically.
func (s *Service) swap(ctx context.Context, out *domain.ModelVersion, outExpected int64, in domain.ModelVersion, inExpected int64, changes []domain.StageChange) error {
	m, err := s.repo.ModelByID(ctx, in.ModelID)
	if err != nil {
		return err
	}
	numbers := map[uuid.UUID]int{in.ID: in.Number}
	if out != nil {
		numbers[out.ID] = out.Number
	}
	records := make([]ports.OutboxRecord, 0, len(changes))
	for _, c := range changes {
		rec, err := s.enc.Encode(domain.StageChanged{ID: s.ids.NewID(), ModelID: m.ID, ModelName: m.Name, Number: numbers[c.VersionID], Change: c})
		if err != nil {
			return err
		}
		records = append(records, rec)
	}
	return s.repo.SwapProduction(ctx, out, outExpected, in, inExpected, changes, records)
}

// ShadowComparison is one ShadowCompared event as the registry stores it.
type ShadowComparison struct {
	ShadowVersionID uuid.UUID
	JobID           string
	Agree           bool
	ShadowMS        float64
}

// RecordShadowComparison stores a comparison (once per job).
func (s *Service) RecordShadowComparison(ctx context.Context, c ShadowComparison) error {
	return s.repo.RecordShadowComparison(ctx, c.ShadowVersionID, c.JobID, c.Agree, c.ShadowMS)
}
