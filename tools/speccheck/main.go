// Command speccheck fails when an OpenSpec requirement has no passing linked test,
// or when a test names a requirement that doesn't exist (decision record 0006).
//
// Usage:
//
//	speccheck -openspec openspec -go go-test.json[,more.json] -junit python-test.xml[,more.xml]
//
// Either report may be omitted; every report given is read and joined.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	specDir := flag.String("openspec", "openspec", "OpenSpec directory")
	goReport := flag.String("go", "", "`go test -json` reports, comma-separated")
	junitReport := flag.String("junit", "", "JUnit XML reports (pytest, Vitest, Playwright), comma-separated")
	flag.Parse()
	if *goReport == "" && *junitReport == "" {
		fail(fmt.Errorf("give at least one of -go and -junit"))
	}

	reqs, err := ReadRequirements(os.DirFS(*specDir))
	if err != nil {
		fail(err)
	}
	type source struct {
		path string
		read func(io.Reader) ([]Link, error)
	}
	var sources []source
	for _, p := range strings.Split(*goReport, ",") {
		sources = append(sources, source{p, ReadGoLinks})
	}
	for _, p := range strings.Split(*junitReport, ",") {
		sources = append(sources, source{p, ReadJUnitLinks})
	}
	var links []Link
	for _, src := range sources {
		if src.path == "" {
			continue
		}
		f, err := os.Open(src.path)
		if err != nil {
			fail(err)
		}
		l, err := src.read(f)
		f.Close()
		if err != nil {
			fail(err)
		}
		links = append(links, l...)
	}

	rep := Check(reqs, links)
	rep.Write(os.Stdout, len(reqs))
	if !rep.OK() {
		os.Exit(1)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "speccheck:", err)
	os.Exit(2)
}
