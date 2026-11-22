package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Pohi11/spotter/internal/spectest"
)

const capability = "build-walkthrough"

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixture is a tiny repository: two archived changes, one spec with three requirements,
// one decision record, and a Go and a JUnit report.
func fixture(t *testing.T) string {
	root := t.TempDir()
	write(t, root, "openspec/changes/archive/2026-01-02-phase-02-api/proposal.md",
		"# Proposal\n\n## Why\n\nClients need an API.\n\n## What Changes\n\n- Add gRPC.\n\n## Impact\n\nSee [0001](../../../../docs/decisions/0001-x.md).\n")
	write(t, root, "openspec/changes/archive/2026-01-02-phase-02-api/tasks.md",
		"# Tasks\n\n## 1. API\n\n- [x] 1.1 Serve gRPC.\n- [ ] 1.2 Later.\n")
	write(t, root, "openspec/changes/archive/2026-01-02-phase-02-api/specs/api/spec.md",
		"## ADDED Requirements\n\n### Requirement: Served\nx\n\n## MODIFIED Requirements\n\n### Requirement: Fast\ny\n")
	write(t, root, "openspec/changes/archive/2026-01-01-phase-01-domain/proposal.md", "## Why\n\nA domain.\n")
	write(t, root, "openspec/specs/api/spec.md", `# api

## Purpose
Serves things.

## Requirements

### Requirement: Served
The API SHALL serve.

#### Scenario: Call
- **WHEN** a client calls
- **THEN** it answers

### Requirement: Fast
It SHALL be fast.

#### Scenario: Quick
- **WHEN** x
- **THEN** y

### Requirement: Documented
It SHALL be documented.

#### Scenario: Docs
- **WHEN** a
- **THEN** b
`)
	write(t, root, "docs/decisions/0001-x.md", "# 0001 — Use gRPC\n\n- **Date:** 2026-01-02\n- **Status:** Accepted\n\n## Context\n")
	write(t, root, "go.json", `{"Action":"output","Package":"p","Test":"TestServed","Output":"spec: api: Served\n"}
{"Action":"pass","Package":"p","Test":"TestServed","Elapsed":0.1}
{"Action":"output","Package":"p","Test":"TestFast","Output":"spec: api: Fast\n"}
{"Action":"fail","Package":"p","Test":"TestFast","Elapsed":0.2}
`)
	write(t, root, "ts.xml", `<testsuites><testsuite><testcase classname="x" name="served too [spec: api: Served]"/></testsuite></testsuites>`)
	return root
}

func build(t *testing.T) *Walkthrough {
	root := fixture(t)
	w, err := Build(root, []Suite{
		{Name: "go", Paths: []string{"go.json"}, Read: readGo},
		{Name: "dashboard", Paths: []string{"ts.xml"}, Read: readJUnit},
		{Name: "e2e", Paths: []string{"missing.xml"}, Read: readJUnit},
	}, time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestRequirementsLinkToTheirTests(t *testing.T) {
	spectest.Verifies(t, capability, "Each requirement links to its tests and their latest results")
	w := build(t)
	got := map[string]Requirement{}
	for _, r := range w.Specs[0].Requirements {
		got[r.Name] = r
	}
	if r := got["Served"]; r.Status != "verified" || len(r.Tests) != 2 {
		t.Errorf("Served = %s with %d tests, want verified with 2 (Go and TypeScript)", r.Status, len(r.Tests))
	}
	if got["Fast"].Status != "failing" || got["Documented"].Status != "untested" {
		t.Errorf("Fast = %s, Documented = %s; want failing and untested", got["Fast"].Status, got["Documented"].Status)
	}
	if s := got["Served"].Scenarios; len(s) != 1 || s[0].Name != "Call" || len(s[0].Steps) != 2 {
		t.Errorf("scenarios %+v", s)
	}
	if w.Totals.Requirements != 3 || w.Totals.Verified != 1 || w.Totals.Tests["go"] != 2 || w.Totals.Tests["dashboard"] != 1 || w.Totals.LinkedTests != 3 {
		t.Errorf("totals %+v", w.Totals)
	}
	if w.Reports["e2e"] != nil || w.Reports["go"] == nil {
		t.Errorf("reports %v: a missing report has no timestamp", w.Reports)
	}
}

func TestEveryArchivedChangeIsOnTheTimeline(t *testing.T) {
	spectest.Verifies(t, capability, "Every archived change is on the timeline")
	w := build(t)
	if len(w.Changes) != 2 || w.Changes[0].Slug != "phase-01-domain" || w.Changes[1].Title != "Phase 2: Api" && w.Changes[1].Title != "Phase 2: API" {
		t.Fatalf("changes %+v", w.Changes)
	}
	c := w.Changes[1]
	if c.Why != "Clients need an API." || c.WhatChanges != "- Add gRPC." {
		t.Errorf("why %q, what %q", c.Why, c.WhatChanges)
	}
	if len(c.Requirements) != 2 || c.Requirements[0] != (RequirementRef{"api", "added", "Served"}) || c.Requirements[1].Kind != "modified" {
		t.Errorf("requirements %+v", c.Requirements)
	}
	if len(c.Decisions) != 1 || c.Decisions[0] != "0001" {
		t.Errorf("decisions %v", c.Decisions)
	}
	if len(c.Tasks) != 1 || len(c.Tasks[0].Items) != 2 || !c.Tasks[0].Items[0].Done || c.Tasks[0].Items[1].Done {
		t.Errorf("tasks %+v", c.Tasks)
	}
}

func TestDecisionRecordsAreIncludedWhole(t *testing.T) {
	spectest.Verifies(t, capability, "Decision records are shown in full")
	w := build(t)
	if len(w.Decisions) != 1 {
		t.Fatalf("decisions %+v", w.Decisions)
	}
	d := w.Decisions[0]
	if d.Number != "0001" || d.Title != "Use gRPC" || d.Date != "2026-01-02" || d.Status != "Accepted" || d.Markdown == "" {
		t.Errorf("decision %+v", d)
	}
}

func TestPrivateNotesAreNeverRead(t *testing.T) {
	spectest.Verifies(t, capability, "The walkthrough reads only public sources")
	root := fixture(t)
	const marker = "PRIVATE-MARKER-7f3a"
	write(t, root, "docs/private/notes.md", marker)
	write(t, root, "PLAN.md", marker)
	write(t, root, "CLAUDE.md", marker)
	w, err := Build(root, []Suite{{Name: "go", Paths: []string{"go.json"}, Read: readGo}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), marker) {
		t.Fatal("the walkthrough contains text from a private file")
	}
}
