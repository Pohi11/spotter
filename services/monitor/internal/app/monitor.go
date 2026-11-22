// Package app holds the monitor's use case: observe completions, detect drift per
// version, and publish state changes. It depends only on domain and ports.
package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Pohi11/spotter/services/monitor/internal/domain"
	"github.com/Pohi11/spotter/services/monitor/internal/ports"
)

// Observation is what the monitor needs from one InferenceCompleted event.
type Observation struct {
	Model     string
	VersionID string
	Version   int
	Sample    domain.Sample
	Sequence  uint64 // stream sequence of the event, for de-duplication keys
}

// Monitor keeps one detector per model version.
type Monitor struct {
	refs    ports.ReferenceSource
	pub     ports.Publisher
	metrics ports.Metrics
	clock   ports.Clock
	cfg     domain.Config

	mu        sync.Mutex
	detectors map[string]*domain.Detector
	missing   map[string]bool // versions known to have no reference
}

func NewMonitor(refs ports.ReferenceSource, pub ports.Publisher, metrics ports.Metrics, clock ports.Clock, cfg domain.Config) *Monitor {
	return &Monitor{refs: refs, pub: pub, metrics: metrics, clock: clock, cfg: cfg,
		detectors: map[string]*domain.Detector{}, missing: map[string]bool{}}
}

// Observe adds one completion to its version's window and publishes any drift state
// change. Events for a version without a reference are counted and skipped.
func (m *Monitor) Observe(ctx context.Context, o Observation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	det, ok := m.detectors[o.VersionID]
	if !ok {
		if m.missing[o.VersionID] {
			m.metrics.Skipped(o.Model, o.VersionID)
			return nil
		}
		ref, err := m.refs.Reference(ctx, o.VersionID)
		if errors.Is(err, ports.ErrNoReference) {
			m.missing[o.VersionID] = true
			m.metrics.Skipped(o.Model, o.VersionID)
			return nil
		}
		if err != nil {
			return fmt.Errorf("reference for version %s: %w", o.VersionID, err)
		}
		det = domain.NewDetector(ref, m.cfg)
		m.detectors[o.VersionID] = det
	}

	tr, results := det.Observe(o.Sample)
	for _, r := range results {
		m.metrics.PSI(o.Model, o.VersionID, r.Feature, r.PSI)
	}
	if tr == domain.NoChange {
		return nil
	}
	e := ports.DriftEvent{Transition: tr, Model: o.Model, VersionID: o.VersionID, Version: o.Version,
		Results: results, WindowSize: m.cfg.Window, At: m.clock.Now(),
		DedupKey: fmt.Sprintf("drift-%s-%d-%d", o.VersionID, tr, o.Sequence)}
	if tr == domain.Detected {
		e.Results = m.cfg.Drifted(results)
	}
	return m.pub.Publish(ctx, e)
}

// Drifting reports whether a version is drifting and whether it is tracked at all.
func (m *Monitor) Drifting(versionID string) (drifting, tracked bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.detectors[versionID]
	if !ok {
		return false, false
	}
	return d.Drifting(), true
}
