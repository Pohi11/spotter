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

// Evaluation is one comparison of a version's window with its reference, published
// whether or not the drift state changed.
type Evaluation struct {
	Model      string
	VersionID  string
	Version    int
	Results    []domain.Result // every feature
	WindowSize int
	Drifting   bool // after this evaluation
	At         time.Time
	DedupKey   string
}

// Publisher announces drift state changes and every evaluation.
type Publisher interface {
	Publish(ctx context.Context, e DriftEvent) error
	PublishEvaluation(ctx context.Context, e Evaluation) error
}

// Metrics records what the monitor sees.
type Metrics interface {
	PSI(model, versionID string, feature domain.Feature, psi float64)
	Skipped(model, versionID string)
}

// Clock tells the time.
type Clock interface{ Now() time.Time }
