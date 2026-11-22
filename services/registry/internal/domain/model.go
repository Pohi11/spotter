// Package domain holds the registry's types and rules. It does no I/O and imports
// nothing but the standard library and github.com/google/uuid (checked by archtest).
package domain

import (
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Model is a named family of versions, e.g. "eurosat-classifier".
type Model struct {
	ID          uuid.UUID
	Name        string
	Description string
	CreatedAt   time.Time
}

// Framework is the format of a version's artifact.
type Framework string

const (
	FrameworkPyTorch Framework = "pytorch"
	FrameworkONNX    Framework = "onnx"
)

// ModelVersion is one trained artifact of a model.
type ModelVersion struct {
	ID             uuid.UUID
	ModelID        uuid.UUID
	Number         int // 1, 2, 3, … per model; assigned by the repository
	ArtifactURI    string
	ArtifactSHA256 string
	Framework      Framework
	Metrics        map[string]float64
	Stage          Stage
	Revision       int64 // increases by one on every transition
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// StageChange is one entry in a version's history.
type StageChange struct {
	VersionID uuid.UUID
	From, To  Stage
	Reason    string
	Revision  int64 // the version's revision after the change
	At        time.Time
}

var (
	modelNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{2,62}$`)
	sha256Pattern    = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// NewModel validates and builds a model.
func NewModel(id uuid.UUID, name, description string, at time.Time) (Model, error) {
	if !modelNamePattern.MatchString(name) {
		return Model{}, invalid("name", "must be 3-63 characters: a lowercase letter, then lowercase letters, digits, or hyphens")
	}
	return Model{ID: id, Name: name, Description: description, CreatedAt: at}, nil
}

// NewVersion validates and builds a version in stage registered at revision 1.
// Its Number is zero until the repository assigns it.
func NewVersion(id, modelID uuid.UUID, artifactURI, sha256 string, framework Framework, metrics map[string]float64, at time.Time) (ModelVersion, error) {
	if err := validateArtifactURI(artifactURI); err != nil {
		return ModelVersion{}, err
	}
	if !sha256Pattern.MatchString(sha256) {
		return ModelVersion{}, invalid("artifact_sha256", "must be 64 lowercase hexadecimal characters")
	}
	if framework != FrameworkPyTorch && framework != FrameworkONNX {
		return ModelVersion{}, invalid("framework", `must be "pytorch" or "onnx"`)
	}
	copied := make(map[string]float64, len(metrics))
	for name, value := range metrics {
		if strings.TrimSpace(name) == "" {
			return ModelVersion{}, invalid("metrics", "metric names must not be blank")
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return ModelVersion{}, invalid("metrics", "metric "+name+" must be a finite number")
		}
		copied[name] = value
	}
	return ModelVersion{
		ID:             id,
		ModelID:        modelID,
		ArtifactURI:    artifactURI,
		ArtifactSHA256: sha256,
		Framework:      framework,
		Metrics:        copied,
		Stage:          StageRegistered,
		Revision:       1,
		CreatedAt:      at,
		UpdatedAt:      at,
	}, nil
}

func validateArtifactURI(uri string) error {
	rest, ok := strings.CutPrefix(uri, "s3://")
	if !ok {
		return invalid("artifact_uri", "must start with s3://")
	}
	bucket, key, ok := strings.Cut(rest, "/")
	if !ok || bucket == "" || key == "" || strings.ContainsAny(uri, " \t\r\n") {
		return invalid("artifact_uri", "must look like s3://<bucket>/<key>")
	}
	return nil
}

// Transition moves the version to another stage. It returns the updated version
// (revision increased by one) and the history entry, or an error and changes nothing.
func (v ModelVersion) Transition(to Stage, reason string, at time.Time) (ModelVersion, StageChange, error) {
	if strings.TrimSpace(reason) == "" {
		return v, StageChange{}, invalid("reason", "must not be blank")
	}
	if !CanTransition(v.Stage, to) {
		return v, StageChange{}, &TransitionError{From: v.Stage, To: to}
	}
	change := StageChange{VersionID: v.ID, From: v.Stage, To: to, Reason: reason, Revision: v.Revision + 1, At: at}
	v.Stage = to
	v.Revision++
	v.UpdatedAt = at
	v.Metrics = cloneMetrics(v.Metrics) // the returned value shares no map with the receiver
	return v, change, nil
}

func cloneMetrics(m map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
