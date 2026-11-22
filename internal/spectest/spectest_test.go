package spectest_test

import (
	"testing"

	"github.com/Pohi11/spotter/internal/spectest"
)

func TestLineFormat(t *testing.T) {
	got := spectest.Line("model-lifecycle", "A transition requires a reason")
	want := "spec: model-lifecycle: A transition requires a reason"
	if got != want {
		t.Fatalf("Line = %q, want %q", got, want)
	}
}

// recorder captures Fatalf instead of stopping the real test.
type recorder struct {
	testing.TB
	failed bool
	logged []string
}

func (r *recorder) Helper()               {}
func (r *recorder) Fatalf(string, ...any) { r.failed = true }
func (r *recorder) Log(args ...any)       { r.logged = append(r.logged, args[0].(string)) }

func TestVerifiesRejectsMalformedArguments(t *testing.T) {
	for _, tc := range []struct{ name, capability, requirement string }{
		{"uppercase capability", "Model-Lifecycle", "x"},
		{"blank requirement", "model-lifecycle", "   "},
		{"multi-line requirement", "model-lifecycle", "a\nb"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &recorder{TB: t}
			spectest.Verifies(r, tc.capability, tc.requirement)
			if !r.failed {
				t.Fatal("Verifies accepted malformed arguments")
			}
		})
	}
}

func TestVerifiesLogsTheLine(t *testing.T) {
	r := &recorder{TB: t}
	spectest.Verifies(r, "model-lifecycle", "A transition requires a reason")
	if r.failed || len(r.logged) != 1 || r.logged[0] != spectest.Line("model-lifecycle", "A transition requires a reason") {
		t.Fatalf("failed=%v logged=%v", r.failed, r.logged)
	}
}
