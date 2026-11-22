// Command walkthrough generates the dashboard's build walkthrough from the repository's
// public sources and the latest test reports (spec: build-walkthrough): archived OpenSpec
// changes, current specs, decision records, and every requirement's linked tests and
// results.
//
// Usage (from the repository root, after the test suites have written their reports):
//
//	go run ./tools/walkthrough -go go-test.json,go-kube-test.json \
//	    -python python-test.xml -dashboard dashboard/test-results/junit.xml \
//	    -e2e dashboard/test-results/e2e-junit.xml
//
// Only openspec/, docs/decisions/, the reports, and git history are read.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pohi11/spotter/internal/testreport"
)

func main() {
	repo := flag.String("repo", ".", "repository root")
	goReports := flag.String("go", "go-test.json", "`go test -json` reports, comma-separated")
	python := flag.String("python", "python-test.xml", "pytest JUnit report")
	dashboard := flag.String("dashboard", "dashboard/test-results/junit.xml", "Vitest JUnit report")
	e2e := flag.String("e2e", "dashboard/test-results/e2e-junit.xml", "Playwright JUnit report")
	out := flag.String("out", "dashboard/content/walkthrough.json", "output file")
	flag.Parse()

	suites := []Suite{{Name: "go", Paths: split(*goReports), Read: readGo}}
	for _, s := range []struct{ name, path string }{{"python", *python}, {"dashboard", *dashboard}, {"e2e", *e2e}} {
		suites = append(suites, Suite{Name: s.name, Paths: split(s.path), Read: readJUnit})
	}
	w, err := Build(*repo, suites, time.Now().UTC())
	if err != nil {
		fail(err)
	}
	data, err := json.MarshalIndent(w, "", " ")
	if err != nil {
		fail(err)
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(*out, append(data, '\n'), 0o644); err != nil {
		fail(err)
	}
	t := w.Totals
	fmt.Printf("wrote %s: %d changes, %d of %d requirements verified, %d decision records\n",
		*out, t.Changes, t.Verified, t.Requirements, len(w.Decisions))
}

func split(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func readGo(r io.Reader) ([]testreport.Case, error) { return testreport.ReadGo(r) }

func readJUnit(r io.Reader) ([]testreport.Case, error) {
	cases, _, err := testreport.ReadJUnit(r)
	return cases, err
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "walkthrough:", err)
	os.Exit(1)
}
