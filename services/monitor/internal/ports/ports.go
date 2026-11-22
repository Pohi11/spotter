// Package ports defines what the monitor's use case needs from the outside world.
package ports

import (
	"context"
	"errors"
	"time"

	"github.com/Pohi11/spotter/services/monitor/internal/domain"
)

// ErrNoReference means a version has no reference distribution.
var ErrNoReference = errors.New("version has no reference distribution")

// ReferenceSource loads a version's training-time reference.
type ReferenceSource interface {
	Reference(ctx context.Context, versionID string) (domain.Reference, error)
}

// DriftEvent is a change of drift state for one version.
type DriftEvent struct {
	Transition domain.Transition // Detected or Resolved
	Model      string
	VersionID  string
	Version    int
	Results    []domain.Result // features at/above the threshold for Detected; all for Resolved
	WindowSize int
	At         time.Time
	// DedupKey identifies this transition so a redelivered batch doesn't publish it twice.
	DedupKey string
}

// Publisher announces drift state changes.
type Publisher interface {
	Publish(ctx context.Context, e DriftEvent) error
}

// Metrics records what the monitor sees.
type Metrics interface {
	PSI(model, versionID string, feature domain.Feature, psi float64)
	Skipped(model, versionID string)
}

// Clock tells the time.
type Clock interface{ Now() time.Time }
