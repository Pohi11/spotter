// Command speccheck fails when an OpenSpec requirement has no passing linked test,
// or when a test names a requirement that doesn't exist (decision record 0006).
//
// Usage:
//
//	speccheck -openspec openspec -go go-test.json
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	specDir := flag.String("openspec", "openspec", "OpenSpec directory")
	goReport := flag.String("go", "go-test.json", "`go test -json` report")
	flag.Parse()

	reqs, err := ReadRequirements(os.DirFS(*specDir))
	if err != nil {
		fail(err)
	}
	f, err := os.Open(*goReport)
	if err != nil {
		fail(err)
	}
	defer f.Close()
	links, err := ReadGoLinks(f)
	if err != nil {
		fail(err)
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
