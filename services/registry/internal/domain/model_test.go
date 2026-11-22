package domain_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/services/registry/internal/domain"
)

const registry = "model-registry"

func TestModelNames(t *testing.T) {
	for _, tc := range []struct {
		name string
		ok   bool
	}{
		{"eurosat-classifier", true},
		{"abc", true},
		{"EuroSAT Classifier", false},
		{"ab", false},
		{"1model", false},
		{"model_v1", false},
		{strings.Repeat("a", 64), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spectest.Verifies(t, registry, "Model names are unique and well-formed")
			_, err := domain.NewModel(uuid.New(), tc.name, "", t0)
			if tc.ok && err != nil {
				t.Fatalf("rejected valid name: %v", err)
			}
			if !tc.ok {
				var ve *domain.ValidationError
				if !errors.As(err, &ve) || ve.Field != "name" {
					t.Fatalf("err = %v, want validation error on name", err)
				}
			}
		})
	}
}

func TestVersionValidation(t *testing.T) {
	good := func() (string, string, domain.Framework, map[string]float64) {
		return "s3://models/eurosat/v1.pt", sha, domain.FrameworkPyTorch, map[string]float64{"accuracy": 0.91}
	}
	for _, tc := range []struct {
		name   string
		mutate func(uri, sum *string, fw *domain.Framework, m map[string]float64)
		field  string // empty means valid
	}{
		{"valid", func(*string, *string, *domain.Framework, map[string]float64) {}, ""},
		{"onnx is supported", func(_, _ *string, fw *domain.Framework, _ map[string]float64) { *fw = domain.FrameworkONNX }, ""},
		{"short checksum", func(_, s *string, _ *domain.Framework, _ map[string]float64) { *s = "abc123" }, "artifact_sha256"},
		{"uppercase checksum", func(_, s *string, _ *domain.Framework, _ map[string]float64) {
			*s = "6B86B273FF34FCE19D6B804EFF5A3F5747ADA4EAA22F1D49C01E52DDB7875B4B"
		}, "artifact_sha256"},
		{"tensorflow", func(_, _ *string, fw *domain.Framework, _ map[string]float64) { *fw = "tensorflow" }, "framework"},
		{"local path", func(u, _ *string, _ *domain.Framework, _ map[string]float64) { *u = "/tmp/model.pt" }, "artifact_uri"},
		{"bucket only", func(u, _ *string, _ *domain.Framework, _ map[string]float64) { *u = "s3://models" }, "artifact_uri"},
		{"NaN metric", func(_, _ *string, _ *domain.Framework, m map[string]float64) { m["loss"] = math.NaN() }, "metrics"},
		{"blank metric name", func(_, _ *string, _ *domain.Framework, m map[string]float64) { m[" "] = 1 }, "metrics"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spectest.Verifies(t, registry, "A version records a verifiable artifact")
			uri, sum, fw, metrics := good()
			tc.mutate(&uri, &sum, &fw, metrics)
			v, err := domain.NewVersion(uuid.New(), uuid.New(), uri, sum, fw, metrics, t0)
			if tc.field == "" {
				if err != nil {
					t.Fatalf("rejected valid version: %v", err)
				}
				if v.Stage != domain.StageRegistered || v.Revision != 1 {
					t.Fatalf("new version is %s at revision %d, want registered at 1", v.Stage, v.Revision)
				}
				return
			}
			var ve *domain.ValidationError
			if !errors.As(err, &ve) || ve.Field != tc.field {
				t.Fatalf("err = %v, want validation error on %s", err, tc.field)
			}
		})
	}
}
