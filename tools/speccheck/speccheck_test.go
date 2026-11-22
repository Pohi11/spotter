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

// A pytest --junitxml excerpt: one passing marked test, one failing, one skipped.
const junitXML = `<?xml version="1.0" encoding="utf-8"?>
<testsuites><testsuite name="pytest">
  <testcase classname="workers.trainer.tests.test_splits" name="test_same_seed">
    <properties><property name="spec" value="training-data: Splits are deterministic"/></properties>
  </testcase>
  <testcase classname="workers.trainer.tests.test_metrics" name="test_known">
    <properties><property name="spec" value="model-evaluation: Evaluation reports per-class metrics"/></properties>
    <failure message="boom">assert 1 == 2</failure>
  </testcase>
  <testcase classname="workers.x" name="test_skipped">
    <properties><property name="spec" value="training-data: Splits are stratified and disjoint"/></properties>
    <skipped message="NATS_URL not set"/>
  </testcase>
</testsuite></testsuites>`

func TestReadJUnitLinks(t *testing.T) {
	links, err := ReadJUnitLinks(strings.NewReader(junitXML))
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 3 {
		t.Fatalf("links = %+v, want 3", links)
	}
	want := map[string]bool{
		"training-data: Splits are deterministic":                true,
		"model-evaluation: Evaluation reports per-class metrics": false,
		"training-data: Splits are stratified and disjoint":      false, // skipped is not passing
	}
	for _, l := range links {
		if got, ok := want[l.Requirement.String()]; !ok || got != l.Passed {
			t.Errorf("link %s passed=%v, want %v", l.Requirement, l.Passed, got)
		}
	}
}

func TestCheckPassesWhenEverythingIsCovered(t *testing.T) {
	reqs := []Requirement{{"model-lifecycle", "Only listed stage transitions are allowed"}}
	links := []Link{{Requirement: reqs[0], Test: "p TestA", Passed: true}}
	if rep := Check(reqs, links); !rep.OK() {
		t.Fatalf("report not OK: %+v", rep)
	}
}

func TestReadJUnitLinksFromTestNames(t *testing.T) {
	const vitest = `<?xml version="1.0" encoding="UTF-8" ?>
<testsuites name="vitest tests"><testsuite name="lib/proxy.test.ts">
  <testcase classname="lib/proxy.test.ts" name="forwards listed routes [spec: dashboard: The browser reaches the gateway only through an allowlisted proxy]"></testcase>
  <testcase classname="lib/proxy.test.ts" name="two tags [spec: dashboard: A] [spec: event-feed: B]"><failure message="x"/></testcase>
  <testcase classname="lib/proxy.test.ts" name="no tag"></testcase>
</testsuite></testsuites>`
	links, err := ReadJUnitLinks(strings.NewReader(vitest))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"dashboard: The browser reaches the gateway only through an allowlisted proxy": true,
		"dashboard: A":  false,
		"event-feed: B": false,
	}
	if len(links) != len(want) {
		t.Fatalf("links = %+v, want %d", links, len(want))
	}
	for _, l := range links {
		if got, ok := want[l.Requirement.String()]; !ok || got != l.Passed {
			t.Errorf("link %s passed=%v, want %v", l.Requirement, l.Passed, got)
		}
	}
}
