// Command speccheck fails when an OpenSpec requirement has no passing linked test,
// or when a test names a requirement that doesn't exist (decision record 0006).
//
// Usage:
//
//	speccheck -openspec openspec -go go-test.json -junit python-test.xml
//
// Either report may be omitted; every report given is read and joined.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	specDir := flag.String("openspec", "openspec", "OpenSpec directory")
	goReport := flag.String("go", "", "`go test -json` report")
	junitReport := flag.String("junit", "", "JUnit XML report (pytest --junitxml)")
	flag.Parse()
	if *goReport == "" && *junitReport == "" {
		fail(fmt.Errorf("give at least one of -go and -junit"))
	}

	reqs, err := ReadRequirements(os.DirFS(*specDir))
	if err != nil {
		fail(err)
	}
	var links []Link
	for _, src := range []struct {
		path string
		read func(io.Reader) ([]Link, error)
	}{{*goReport, ReadGoLinks}, {*junitReport, ReadJUnitLinks}} {
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
