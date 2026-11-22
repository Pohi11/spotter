package main

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Pohi11/spotter/internal/testreport"
)

// Requirement identifies one "### Requirement:" block in a spec.
type Requirement struct {
	Capability string
	Title      string
}

func (r Requirement) String() string { return r.Capability + ": " + r.Title }

var requirementHeading = regexp.MustCompile(`^###\s+Requirement:\s*(.+?)\s*$`)

// ReadRequirements collects requirements from openspec/specs/<capability>/spec.md and
// from active changes, openspec/changes/<change>/specs/<capability>/spec.md.
// Archived changes are skipped: their requirements already live in openspec/specs.
func ReadRequirements(fsys fs.FS) ([]Requirement, error) {
	seen := map[Requirement]bool{}
	var out []Requirement
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "archive" {
			return fs.SkipDir
		}
		if d.IsDir() || d.Name() != "spec.md" {
			return nil
		}
		capability := filepath.Base(filepath.Dir(filepath.FromSlash(path)))
		f, err := fsys.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if m := requirementHeading.FindStringSubmatch(sc.Text()); m != nil {
				r := Requirement{Capability: capability, Title: m[1]}
				if !seen[r] {
					seen[r] = true
					out = append(out, r)
				}
			}
		}
		return sc.Err()
	})
	return out, err
}

// Link is a test that named a requirement, with its final result.
type Link struct {
	Requirement Requirement
	Test        string // "package TestName/subtest" or "classname name"
	Passed      bool
}

// links turns test cases into requirement links (reports are read by internal/testreport).
func links(cases []testreport.Case) ([]Link, error) {
	var out []Link
	for _, c := range cases {
		for _, r := range c.Requirements {
			capability, title, ok := strings.Cut(r, ": ")
			if !ok {
				return nil, fmt.Errorf("test %s names malformed requirement %q", c.ID, r)
			}
			out = append(out, Link{Requirement: Requirement{Capability: capability, Title: title}, Test: c.ID, Passed: c.Status == "passed"})
		}
	}
	return out, nil
}

// ReadGoLinks parses a `go test -json` stream into requirement links.
func ReadGoLinks(r io.Reader) ([]Link, error) {
	cases, err := testreport.ReadGo(r)
	if err != nil {
		return nil, err
	}
	return links(cases)
}

// ReadJUnitLinks parses a JUnit XML report into requirement links: pytest's spec
// properties and requirement tags in test names (decision record 0017).
func ReadJUnitLinks(r io.Reader) ([]Link, error) {
	cases, _, err := testreport.ReadJUnit(r)
	if err != nil {
		return nil, err
	}
	return links(cases)
}

// Report is the result of joining requirements with test links.
type Report struct {
	Covered   map[Requirement][]string // requirement → passing tests
	Uncovered []Requirement            // no passing linked test
	Unknown   []Link                   // tests naming a requirement that doesn't exist
}

// OK reports whether every requirement is covered and every link is known.
func (r Report) OK() bool { return len(r.Uncovered) == 0 && len(r.Unknown) == 0 }

// Check joins requirements with links.
func Check(reqs []Requirement, links []Link) Report {
	known := map[Requirement]bool{}
	for _, r := range reqs {
		known[r] = true
	}
	rep := Report{Covered: map[Requirement][]string{}}
	for _, l := range links {
		if !known[l.Requirement] {
			rep.Unknown = append(rep.Unknown, l)
			continue
		}
		if l.Passed {
			rep.Covered[l.Requirement] = append(rep.Covered[l.Requirement], l.Test)
		}
	}
	for _, r := range reqs {
		if len(rep.Covered[r]) == 0 {
			rep.Uncovered = append(rep.Uncovered, r)
		}
	}
	sort.Slice(rep.Unknown, func(i, j int) bool { return rep.Unknown[i].Test < rep.Unknown[j].Test })
	return rep
}

// Write prints a human-readable summary.
func (r Report) Write(w io.Writer, total int) {
	fmt.Fprintf(w, "spec coverage: %d of %d requirements have a passing linked test\n", total-len(r.Uncovered), total)
	for _, req := range r.Uncovered {
		fmt.Fprintf(w, "  UNCOVERED  %s\n", req)
	}
	for _, l := range r.Unknown {
		fmt.Fprintf(w, "  UNKNOWN    %s (named by %s)\n", l.Requirement, strings.TrimSpace(l.Test))
	}
}
