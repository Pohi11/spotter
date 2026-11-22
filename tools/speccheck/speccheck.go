package main

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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

// testEvent is one line of `go test -json` output.
type testEvent struct {
	Action  string
	Package string
	Test    string
	Output  string
}

// Link is a test that named a requirement, with its final result.
type Link struct {
	Requirement Requirement
	Test        string // "package TestName/subtest"
	Passed      bool
}

var specLine = regexp.MustCompile(`spec: ([a-z][a-z0-9-]*): (.+?)\s*$`)

// ReadGoLinks parses a `go test -json` stream into requirement links.
func ReadGoLinks(r io.Reader) ([]Link, error) {
	type key struct{ pkg, test string }
	named := map[key][]Requirement{}
	result := map[key]string{}

	dec := json.NewDecoder(r)
	for {
		var ev testEvent
		if err := dec.Decode(&ev); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("read go test -json: %w", err)
		}
		if ev.Test == "" {
			continue
		}
		k := key{ev.Package, ev.Test}
		switch ev.Action {
		case "output":
			if m := specLine.FindStringSubmatch(ev.Output); m != nil {
				named[k] = append(named[k], Requirement{Capability: m[1], Title: m[2]})
			}
		case "pass", "fail", "skip":
			result[k] = ev.Action
		}
	}

	var links []Link
	for k, reqs := range named {
		for _, req := range reqs {
			links = append(links, Link{Requirement: req, Test: k.pkg + " " + k.test, Passed: result[k] == "pass"})
		}
	}
	return links, nil
}

// junitSuites matches pytest's JUnit XML: <testsuites><testsuite><testcase> with
// <properties><property name="spec" value="capability: requirement"/></properties>.
type junitSuites struct {
	Suites []struct {
		Cases []junitCase `xml:"testcase"`
	} `xml:"testsuite"`
}

type junitCase struct {
	Class      string `xml:"classname,attr"`
	Name       string `xml:"name,attr"`
	Properties []struct {
		Name  string `xml:"name,attr"`
		Value string `xml:"value,attr"`
	} `xml:"properties>property"`
	Failure *struct{} `xml:"failure"`
	Error   *struct{} `xml:"error"`
	Skipped *struct{} `xml:"skipped"`
}

// ReadJUnitLinks parses a JUnit XML report (pytest --junitxml) into requirement links.
func ReadJUnitLinks(r io.Reader) ([]Link, error) {
	var doc junitSuites
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("read junit xml: %w", err)
	}
	var links []Link
	for _, s := range doc.Suites {
		for _, c := range s.Cases {
			passed := c.Failure == nil && c.Error == nil && c.Skipped == nil
			for _, p := range c.Properties {
				if p.Name != "spec" {
					continue
				}
				capability, title, ok := strings.Cut(p.Value, ": ")
				if !ok {
					return nil, fmt.Errorf("junit: test %s.%s has malformed spec property %q", c.Class, c.Name, p.Value)
				}
				links = append(links, Link{Requirement: Requirement{Capability: capability, Title: title}, Test: c.Class + " " + c.Name, Passed: passed})
			}
		}
	}
	return links, nil
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
