// Package testreport reads test reports from every language in the repository and the
// requirement each test names (decision records 0006 and 0017): `go test -json` streams,
// where spectest.Verifies logs "spec: <capability>: <requirement>", and JUnit XML, where
// pytest writes a "spec" property and Vitest and Playwright carry "[spec: …]" in the name.
package testreport

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Case is one test and its result.
type Case struct {
	ID           string   `json:"id"`
	Status       string   `json:"status"` // passed, failed, or skipped
	Seconds      float64  `json:"seconds"`
	Requirements []string `json:"-"` // "capability: requirement"
}

var (
	goSpecLine = regexp.MustCompile(`spec: ([a-z][a-z0-9-]*): (.+?)\s*$`)
	nameTag    = regexp.MustCompile(`\s*\[spec: ([a-z][a-z0-9-]*): ([^\]]+?)\]`)
)

type goEvent struct {
	Action  string
	Package string
	Test    string
	Output  string
	Elapsed float64
}

// ReadGo parses a `go test -json` stream. Subtests are cases of their own.
func ReadGo(r io.Reader) ([]Case, error) {
	type key struct{ pkg, test string }
	cases := map[key]*Case{}
	var order []key
	dec := json.NewDecoder(r)
	for {
		var ev goEvent
		if err := dec.Decode(&ev); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("read go test -json: %w", err)
		}
		if ev.Test == "" {
			continue
		}
		k := key{ev.Package, ev.Test}
		c, ok := cases[k]
		if !ok {
			c = &Case{ID: ev.Package + " " + ev.Test}
			cases[k] = c
			order = append(order, k)
		}
		switch ev.Action {
		case "output":
			if m := goSpecLine.FindStringSubmatch(ev.Output); m != nil {
				c.Requirements = append(c.Requirements, m[1]+": "+m[2])
			}
		case "pass":
			c.Status, c.Seconds = "passed", ev.Elapsed
		case "fail":
			c.Status, c.Seconds = "failed", ev.Elapsed
		case "skip":
			c.Status, c.Seconds = "skipped", ev.Elapsed
		}
	}
	out := make([]Case, 0, len(order))
	for _, k := range order {
		c := cases[k]
		if c.Status == "" {
			c.Status = "failed" // started but never finished: a crash or a timeout
		}
		out = append(out, *c)
	}
	return out, nil
}

type junitDoc struct {
	Suites []junitSuite `xml:"testsuite"`
	Cases  []junitCase  `xml:"testcase"` // a report whose root is one <testsuite>
}

type junitSuite struct {
	Timestamp string       `xml:"timestamp,attr"`
	Suites    []junitSuite `xml:"testsuite"`
	Cases     []junitCase  `xml:"testcase"`
}

type junitCase struct {
	Class      string `xml:"classname,attr"`
	Name       string `xml:"name,attr"`
	Time       string `xml:"time,attr"`
	Properties []struct {
		Name  string `xml:"name,attr"`
		Value string `xml:"value,attr"`
	} `xml:"properties>property"`
	Failure *struct{} `xml:"failure"`
	Error   *struct{} `xml:"error"`
	Skipped *struct{} `xml:"skipped"`
}

// ReadJUnit parses a JUnit XML report and returns its cases and its latest suite timestamp.
func ReadJUnit(r io.Reader) ([]Case, string, error) {
	var doc junitDoc
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, "", fmt.Errorf("read junit xml: %w", err)
	}
	var out []Case
	var stamps []string
	var walk func(cases []junitCase, suites []junitSuite) error
	walk = func(cases []junitCase, suites []junitSuite) error {
		for _, c := range cases {
			tc, err := junitCase2Case(c)
			if err != nil {
				return err
			}
			out = append(out, tc)
		}
		for _, s := range suites {
			if s.Timestamp != "" {
				stamps = append(stamps, s.Timestamp)
			}
			if err := walk(s.Cases, s.Suites); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(doc.Cases, doc.Suites); err != nil {
		return nil, "", err
	}
	sort.Strings(stamps)
	latest := ""
	if len(stamps) > 0 {
		latest = stamps[len(stamps)-1]
	}
	return out, latest, nil
}

func junitCase2Case(c junitCase) (Case, error) {
	tc := Case{Status: "passed"}
	switch {
	case c.Failure != nil || c.Error != nil:
		tc.Status = "failed"
	case c.Skipped != nil:
		tc.Status = "skipped"
	}
	tc.Seconds, _ = strconv.ParseFloat(c.Time, 64)
	for _, p := range c.Properties {
		if p.Name != "spec" {
			continue
		}
		if _, _, ok := strings.Cut(p.Value, ": "); !ok {
			return Case{}, fmt.Errorf("junit: test %s.%s has malformed spec property %q", c.Class, c.Name, p.Value)
		}
		tc.Requirements = append(tc.Requirements, p.Value)
	}
	for _, m := range nameTag.FindAllStringSubmatch(c.Name, -1) {
		tc.Requirements = append(tc.Requirements, m[1]+": "+m[2])
	}
	tc.ID = c.Class + " " + strings.TrimSpace(nameTag.ReplaceAllString(c.Name, ""))
	return tc, nil
}
