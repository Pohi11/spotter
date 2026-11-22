// Package spectest links Go tests to the OpenSpec requirements they verify.
//
// A test calls Verifies at its start. The call logs one structured line, which
// `go test -json` records in that test's output events, next to its pass/fail
// event. tools/speccheck joins those lines with the specs (decision record 0006).
package spectest

import (
	"regexp"
	"strings"
	"testing"
)

// Prefix starts every line Verifies logs. speccheck looks for it.
const Prefix = "spec: "

var capabilityPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Verifies records that the calling test verifies the named requirement.
// capability is the spec folder name under openspec/specs; requirement is the
// exact text after "### Requirement: " in that spec.
func Verifies(t testing.TB, capability, requirement string) {
	t.Helper()
	if !capabilityPattern.MatchString(capability) {
		t.Fatalf("spectest: capability %q must be a lowercase slug like %q", capability, "model-lifecycle")
	}
	if strings.TrimSpace(requirement) == "" || strings.ContainsAny(requirement, "\r\n") {
		t.Fatalf("spectest: requirement must be a single non-blank line, got %q", requirement)
	}
	t.Log(Line(capability, requirement))
}

// Line is the exact text Verifies logs.
func Line(capability, requirement string) string {
	return Prefix + capability + ": " + requirement
}
