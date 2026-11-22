package main

import (
	"strings"
	"testing"
	"testing/fstest"
)

const specBody = `# Spec Delta

## ADDED Requirements

### Requirement: Only listed stage transitions are allowed
text

#### Scenario: x
- **WHEN** a
- **THEN** b

### Requirement: A transition requires a reason
text
`

func TestReadRequirementsSkipsArchivedChanges(t *testing.T) {
	fsys := fstest.MapFS{
		"specs/model-lifecycle/spec.md":                                {Data: []byte(specBody)},
		"changes/phase-02/specs/model-api/spec.md":                     {Data: []byte("### Requirement: Versions are served over gRPC\n")},
		"changes/archive/2026-01-01-old/specs/model-lifecycle/spec.md": {Data: []byte("### Requirement: Old wording\n")},
	}
	reqs, err := ReadRequirements(fsys)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, r := range reqs {
		got[r.String()] = true
	}
	for _, want := range []string{
		"model-lifecycle: Only listed stage transitions are allowed",
		"model-lifecycle: A transition requires a reason",
		"model-api: Versions are served over gRPC",
	} {
		if !got[want] {
			t.Errorf("missing %q in %v", want, got)
		}
	}
	if got["model-lifecycle: Old wording"] {
		t.Error("read a requirement from an archived change")
	}
}

// A real `go test -json` excerpt: one passing test with a link, one failing test with a link,
// and one subtest with a link.
const goJSON = `{"Action":"run","Package":"p","Test":"TestA"}
{"Action":"output","Package":"p","Test":"TestA","Output":"    a_test.go:10: spec: model-lifecycle: Only listed stage transitions are allowed\n"}
{"Action":"pass","Package":"p","Test":"TestA"}
{"Action":"run","Package":"p","Test":"TestB"}
{"Action":"output","Package":"p","Test":"TestB","Output":"    b_test.go:5: spec: model-lifecycle: A transition requires a reason\n"}
{"Action":"fail","Package":"p","Test":"TestB"}
{"Action":"run","Package":"p","Test":"TestC/case_1"}
{"Action":"output","Package":"p","Test":"TestC/case_1","Output":"    c_test.go:7: spec: model-lifecycle: No such requirement\n"}
{"Action":"pass","Package":"p","Test":"TestC/case_1"}
{"Action":"pass","Package":"p"}
`

func TestCheckReportsUncoveredAndUnknown(t *testing.T) {
	reqs, err := ReadRequirements(fstest.MapFS{"specs/model-lifecycle/spec.md": {Data: []byte(specBody)}})
	if err != nil {
		t.Fatal(err)
	}
	links, err := ReadGoLinks(strings.NewReader(goJSON))
	if err != nil {
		t.Fatal(err)
	}
	rep := Check(reqs, links)

	if rep.OK() {
		t.Fatal("report is OK, want uncovered and unknown entries")
	}
	if len(rep.Covered[Requirement{"model-lifecycle", "Only listed stage transitions are allowed"}]) != 1 {
		t.Errorf("passing linked test not counted: %v", rep.Covered)
	}
	if len(rep.Uncovered) != 1 || rep.Uncovered[0].Title != "A transition requires a reason" {
		t.Errorf("uncovered = %v, want only the requirement whose test failed", rep.Uncovered)
	}
	if len(rep.Unknown) != 1 || rep.Unknown[0].Test != "p TestC/case_1" {
		t.Errorf("unknown = %v, want the subtest naming a missing requirement", rep.Unknown)
	}
}

func TestCheckPassesWhenEverythingIsCovered(t *testing.T) {
	reqs := []Requirement{{"model-lifecycle", "Only listed stage transitions are allowed"}}
	links := []Link{{Requirement: reqs[0], Test: "p TestA", Passed: true}}
	if rep := Check(reqs, links); !rep.OK() {
		t.Fatalf("report not OK: %+v", rep)
	}
}
