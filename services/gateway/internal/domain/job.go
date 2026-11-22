// Package domain holds the gateway's inference job types and submission rules.
// No I/O; checked by archtest.
package domain

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// MaxImageBytes is the largest image accepted for inference.
const MaxImageBytes = 1 << 20

// Status is where a job is.
type Status string

const (
	StatusPending   Status = "pending"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

// ClassScore is the model's score for one class.
type ClassScore struct {
	Label string
	Score float32
}

// Prediction is a finished job's result.
type Prediction struct {
	Label          string
	Confidence     float32
	Scores         []ClassScore
	ModelVersionID string
	ModelVersion   int
}

// Job is one inference request and, once finished, its outcome.
type Job struct {
	ID          uuid.UUID
	ModelName   string
	Status      Status
	ImageURI    string
	SubmittedAt time.Time
	CompletedAt time.Time   // zero while pending
	Prediction  *Prediction // set when succeeded
	Error       string      // set when failed
}

// Errors returned by the gateway's use cases and adapters.
var (
	ErrNotFound    = errors.New("not found")
	ErrInvalid     = errors.New("invalid input")
	ErrUnavailable = errors.New("a dependency is unavailable")
	// ErrNotConfigured: an optional capability (synchronous classification) is off.
	ErrNotConfigured = errors.New("not configured")
)

// ValidationError names the offending field. errors.Is(err, ErrInvalid) is true.
type ValidationError struct {
	Field, Problem string
}

func (e *ValidationError) Error() string { return fmt.Sprintf("invalid %s: %s", e.Field, e.Problem) }
func (e *ValidationError) Unwrap() error { return ErrInvalid }

var modelNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{2,62}$`)

// ValidateSubmission checks a submission before anything is stored or published.
func ValidateSubmission(modelName string, image []byte) error {
	if !modelNamePattern.MatchString(modelName) {
		return &ValidationError{Field: "model_name", Problem: "must be a valid model name"}
	}
	if len(image) == 0 {
		return &ValidationError{Field: "image", Problem: "must not be empty"}
	}
	if len(image) > MaxImageBytes {
		return &ValidationError{Field: "image", Problem: fmt.Sprintf("must be at most %d bytes, got %d", MaxImageBytes, len(image))}
	}
	return nil
}
