package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Pohi11/spotter/internal/testreport"
)

// Suite is one kind of test report; a suite may have several files (Go's kind cluster run).
type Suite struct {
	Name  string
	Paths []string
	Read  func(io.Reader) ([]testreport.Case, error)
}

type Walkthrough struct {
	GeneratedAt string             `json:"generated_at"`
	Reports     map[string]*string `json:"reports"`
	Totals      Totals             `json:"totals"`
	Changes     []Change           `json:"changes"`
	Specs       []Capability       `json:"specs"`
	Decisions   []Decision         `json:"decisions"`
}

type Totals struct {
	Changes      int            `json:"changes"`
	Requirements int            `json:"requirements"`
	Verified     int            `json:"verified"`
	Tests        map[string]int `json:"tests"`
	LinkedTests  int            `json:"linked_tests"`
}

type Change struct {
	ID           string           `json:"id"`
	Slug         string           `json:"slug"`
	Title        string           `json:"title"`
	Date         string           `json:"date"`
	ArchivedAt   *string          `json:"archived_at"`
	Why          string           `json:"why"`
	WhatChanges  string           `json:"what_changes"`
	Tasks        []TaskGroup      `json:"tasks"`
	Requirements []RequirementRef `json:"requirements"`
	Decisions    []string         `json:"decisions"`
}

type TaskGroup struct {
	Title string `json:"title"`
	Items []Task `json:"items"`
}

type Task struct {
	ID   string `json:"id"`
	Done bool   `json:"done"`
	Text string `json:"text"`
}

type RequirementRef struct {
	Capability string `json:"capability"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
}

type Capability struct {
	Capability   string        `json:"capability"`
	Purpose      string        `json:"purpose"`
	Requirements []Requirement `json:"requirements"`
}

type Scenario struct {
	Name  string   `json:"name"`
	Steps []string `json:"steps"`
}

type Requirement struct {
	Name      string       `json:"name"`
	Text      string       `json:"text"`
	Scenarios []Scenario   `json:"scenarios"`
	Tests     []LinkedTest `json:"tests"`
	Status    string       `json:"status"` // verified, failing, or untested
}

type LinkedTest struct {
	Suite   string  `json:"suite"`
	ID      string  `json:"id"`
	Status  string  `json:"status"`
	Seconds float64 `json:"seconds"`
}

type Decision struct {
	Number   string `json:"number"`
	Title    string `json:"title"`
	Date     string `json:"date"`
	Status   string `json:"status"`
	Markdown string `json:"markdown"`
}

// Build reads the sources under repo and joins requirements with their tests.
func Build(repo string, suites []Suite, now time.Time) (*Walkthrough, error) {
	w := &Walkthrough{GeneratedAt: now.Format(time.RFC3339), Reports: map[string]*string{}, Totals: Totals{Tests: map[string]int{}}}
	byRequirement := map[string][]LinkedTest{}
	for _, s := range suites {
		w.Totals.Tests[s.Name] = 0
		var latest time.Time
		for _, p := range s.Paths {
			f, err := os.Open(filepath.Join(repo, p))
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			info, _ := f.Stat()
			cases, err := s.Read(f)
			f.Close()
			if err != nil {
				return nil, err
			}
			if info != nil && info.ModTime().After(latest) {
				latest = info.ModTime()
			}
			for _, c := range cases {
				w.Totals.Tests[s.Name]++
				if len(c.Requirements) > 0 {
					w.Totals.LinkedTests++
				}
				for _, r := range c.Requirements {
					byRequirement[r] = append(byRequirement[r], LinkedTest{Suite: s.Name, ID: c.ID, Status: c.Status, Seconds: c.Seconds})
				}
			}
		}
		if !latest.IsZero() {
			stamp := latest.UTC().Format(time.RFC3339)
			w.Reports[s.Name] = &stamp
		} else {
			w.Reports[s.Name] = nil
		}
	}

	specs, err := readSpecs(filepath.Join(repo, "openspec", "specs"))
	if err != nil {
		return nil, err
	}
	for ci := range specs {
		for ri := range specs[ci].Requirements {
			r := &specs[ci].Requirements[ri]
			r.Tests = byRequirement[specs[ci].Capability+": "+r.Name]
			if r.Tests == nil {
				r.Tests = []LinkedTest{}
			}
			r.Status = status(r.Tests)
			w.Totals.Requirements++
			if r.Status == "verified" {
				w.Totals.Verified++
			}
		}
	}
	w.Specs = specs
	if w.Changes, err = readChanges(filepath.Join(repo, "openspec", "changes", "archive")); err != nil {
		return nil, err
	}
	w.Totals.Changes = len(w.Changes)
	if w.Decisions, err = readDecisions(filepath.Join(repo, "docs", "decisions")); err != nil {
		return nil, err
	}
	return w, nil
}

// status: verified when every linked test passed (skips don't count against it, but at
// least one must have passed), failing when any failed, untested otherwise.
func status(tests []LinkedTest) string {
	passed := false
	for _, t := range tests {
		if t.Status == "failed" {
			return "failing"
		}
		if t.Status == "passed" {
			passed = true
		}
	}
	if passed {
		return "verified"
	}
	return "untested"
}

// --- Markdown -----------------------------------------------------------------------------

var (
	requirementHead = regexp.MustCompile(`(?m)^### Requirement: (.+)$`)
	scenarioHead    = regexp.MustCompile(`(?m)^#### Scenario: (.+)$`)
	deltaHead       = regexp.MustCompile(`(?m)^## (ADDED|MODIFIED|REMOVED|RENAMED) Requirements\s*$`)
	taskLine        = regexp.MustCompile(`^- \[([ xX])\] (\S+) (.*)$`)
	decisionLink    = regexp.MustCompile(`decisions/(\d{4})-`)
	decisionTitle   = regexp.MustCompile(`(?m)^# (\d{4}) — (.+)$`)
	decisionMeta    = regexp.MustCompile(`(?m)^- \*\*(\w+):\*\* (.+)$`)
	phaseSlug       = regexp.MustCompile(`^phase-(\d+)([a-z]?)-(.*)$`)
)

func read(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

// section returns the text under "## heading", up to the next "## ".
func section(md, heading string) string {
	lines := strings.Split(md, "\n")
	var out []string
	in := false
	for _, l := range lines {
		if strings.HasPrefix(l, "## ") {
			if in {
				break
			}
			in = strings.TrimSpace(l[3:]) == heading
			continue
		}
		if in {
			out = append(out, l)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func requirements(md string) []Requirement {
	idx := requirementHead.FindAllStringSubmatchIndex(md, -1)
	var out []Requirement
	for i, m := range idx {
		end := len(md)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		body := md[m[1]:end]
		if j := strings.Index(body, "\n## "); j >= 0 {
			body = body[:j]
		}
		r := Requirement{Name: strings.TrimSpace(md[m[2]:m[3]]), Scenarios: []Scenario{}}
		sc := scenarioHead.FindAllStringSubmatchIndex(body, -1)
		if len(sc) == 0 {
			r.Text = strings.TrimSpace(body)
		} else {
			r.Text = strings.TrimSpace(body[:sc[0][0]])
		}
		for k, s := range sc {
			e := len(body)
			if k+1 < len(sc) {
				e = sc[k+1][0]
			}
			var steps []string
			for _, l := range strings.Split(body[s[1]:e], "\n") {
				if l = strings.TrimSpace(l); strings.HasPrefix(l, "- ") {
					steps = append(steps, l[2:])
				}
			}
			r.Scenarios = append(r.Scenarios, Scenario{Name: strings.TrimSpace(body[s[2]:s[3]]), Steps: steps})
		}
		out = append(out, r)
	}
	return out
}

func readSpecs(dir string) ([]Capability, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []Capability
	for _, e := range entries {
		md := read(filepath.Join(dir, e.Name(), "spec.md"))
		if !e.IsDir() || md == "" {
			continue
		}
		out = append(out, Capability{Capability: e.Name(), Purpose: section(md, "Purpose"), Requirements: requirements(section(md, "Requirements"))})
	}
	return out, nil
}

func humanize(slug string) string {
	words := func(s string) string {
		w := strings.ReplaceAll(s, "-", " ")
		for from, to := range map[string]string{"api": "API", "ci": "CI", "cd": "CD"} {
			w = regexp.MustCompile(`\b`+from+`\b`).ReplaceAllString(w, to)
		}
		w = strings.ReplaceAll(w, "event driven", "event-driven")
		return strings.ToUpper(w[:1]) + w[1:]
	}
	if m := phaseSlug.FindStringSubmatch(slug); m != nil {
		n, _ := strconv.Atoi(m[1])
		return "Phase " + strconv.Itoa(n) + m[2] + ": " + words(m[3])
	}
	return words(slug)
}

// archivedAt is when the change's folder first appeared in git history, so changes archived
// on the same day keep their commit order.
func archivedAt(folder string) *string {
	out, err := exec.Command("git", "-C", folder, "log", "--no-renames", "--diff-filter=A", "--format=%cI", "--", ".").Output()
	if err != nil {
		return nil
	}
	stamps := strings.Fields(string(out))
	if len(stamps) == 0 {
		return nil
	}
	sort.Strings(stamps)
	return &stamps[0]
}

func readChanges(archive string) ([]Change, error) {
	entries, err := os.ReadDir(archive)
	if err != nil {
		return nil, err
	}
	var out []Change
	for _, e := range entries {
		if !e.IsDir() || len(e.Name()) < 12 {
			continue
		}
		dir := filepath.Join(archive, e.Name())
		proposal, design, tasks := read(filepath.Join(dir, "proposal.md")), read(filepath.Join(dir, "design.md")), read(filepath.Join(dir, "tasks.md"))
		c := Change{
			ID: e.Name(), Slug: e.Name()[11:], Title: humanize(e.Name()[11:]), Date: e.Name()[:10],
			ArchivedAt: archivedAt(dir), Why: section(proposal, "Why"), WhatChanges: section(proposal, "What Changes"),
			Tasks: taskGroups(tasks), Requirements: []RequirementRef{}, Decisions: []string{},
		}
		specDirs, _ := os.ReadDir(filepath.Join(dir, "specs"))
		for _, sd := range specDirs {
			md := read(filepath.Join(dir, "specs", sd.Name(), "spec.md"))
			heads := deltaHead.FindAllStringSubmatch(md, -1)
			for _, h := range heads {
				body := section(md, h[1]+" Requirements")
				for _, m := range requirementHead.FindAllStringSubmatch(body, -1) {
					c.Requirements = append(c.Requirements, RequirementRef{Capability: sd.Name(), Kind: strings.ToLower(h[1]), Name: strings.TrimSpace(m[1])})
				}
			}
		}
		seen := map[string]bool{}
		for _, m := range decisionLink.FindAllStringSubmatch(proposal+design+tasks, -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				c.Decisions = append(c.Decisions, m[1])
			}
		}
		sort.Strings(c.Decisions)
		out = append(out, c)
	}
	key := func(c Change) string {
		if c.ArchivedAt != nil {
			return *c.ArchivedAt
		}
		return c.Date + "T99"
	}
	sort.SliceStable(out, func(i, j int) bool {
		if key(out[i]) != key(out[j]) {
			return key(out[i]) < key(out[j])
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func taskGroups(md string) []TaskGroup {
	var out []TaskGroup
	for _, l := range strings.Split(md, "\n") {
		if strings.HasPrefix(l, "## ") {
			out = append(out, TaskGroup{Title: strings.TrimSpace(l[3:])})
			continue
		}
		if m := taskLine.FindStringSubmatch(l); m != nil && len(out) > 0 {
			out[len(out)-1].Items = append(out[len(out)-1].Items, Task{ID: m[2], Done: m[1] != " ", Text: strings.TrimSpace(m[3])})
		}
	}
	kept := out[:0]
	for _, g := range out {
		if len(g.Items) > 0 {
			kept = append(kept, g)
		}
	}
	return kept
}

func readDecisions(dir string) ([]Decision, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []Decision
	for _, e := range entries {
		name := e.Name()
		if len(name) < 6 || !strings.HasSuffix(name, ".md") || strings.Trim(name[:4], "0123456789") != "" {
			continue
		}
		md := read(filepath.Join(dir, name))
		d := Decision{Number: name[:4], Title: strings.TrimSuffix(name[5:], ".md"), Markdown: md}
		if m := decisionTitle.FindStringSubmatch(md); m != nil {
			d.Title = strings.TrimSpace(m[2])
		}
		for _, m := range decisionMeta.FindAllStringSubmatch(md, -1) {
			switch m[1] {
			case "Date":
				d.Date = strings.TrimSpace(m[2])
			case "Status":
				d.Status = strings.TrimSpace(m[2])
			}
		}
		out = append(out, d)
	}
	return out, nil
}
