// Package archtest enforces the hexagonal dependency rule for the monitor: domain,
// ports, and app may not depend on infrastructure (design D9).
package archtest

import (
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"testing"
)

const module = "github.com/Pohi11/spotter"

// core lists each checked package and the module packages it may import.
var core = map[string][]string{
	module + "/services/monitor/internal/domain": {},
	module + "/services/monitor/internal/ports":  {module + "/services/monitor/internal/domain"},
	module + "/services/monitor/internal/app": {
		module + "/services/monitor/internal/domain",
		module + "/services/monitor/internal/ports",
	},
}

// allowedThirdParty is the only non-standard-library module core code may use.
const allowedThirdParty = "github.com/google/uuid"

// Standard-library packages core code must not import directly.
var forbiddenStd = []string{"net", "database", "os/exec", "syscall", "plugin"}

type pkg struct {
	ImportPath string
	Standard   bool
	Imports    []string
	Deps       []string
}

func list(t *testing.T, path string) map[string]pkg {
	t.Helper()
	out, err := exec.Command("go", "list", "-deps", "-json", path).Output()
	if err != nil {
		t.Fatalf("go list %s: %v", path, err)
	}
	pkgs := map[string]pkg{}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p pkg
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		pkgs[p.ImportPath] = p
	}
	return pkgs
}

func TestCoreDoesNotDependOnInfrastructure(t *testing.T) {
	for path, allowedModule := range core {
		t.Run(strings.TrimPrefix(path, module+"/"), func(t *testing.T) {
			// An engineering rule for every Go service; the monitor has no spec requirement for it.
			for _, problem := range violations(list(t, path), path, allowedModule) {
				t.Error(problem)
			}
		})
	}
}

// violations returns every way path breaks the rule.
func violations(pkgs map[string]pkg, path string, allowedModule []string) []string {
	var problems []string
	self := pkgs[path]

	for _, imp := range self.Imports {
		if p, ok := pkgs[imp]; ok && p.Standard {
			for _, bad := range forbiddenStd {
				if imp == bad || strings.HasPrefix(imp, bad+"/") {
					problems = append(problems, path+" imports "+imp)
				}
			}
		}
	}
	allowed := map[string]bool{path: true}
	for _, a := range allowedModule {
		allowed[a] = true
	}
	for _, dep := range self.Deps {
		p := pkgs[dep]
		switch {
		case p.Standard:
		case strings.HasPrefix(dep, module+"/"):
			if !allowed[dep] {
				problems = append(problems, path+" depends on "+dep)
			}
		case dep == allowedThirdParty || strings.HasPrefix(dep, allowedThirdParty+"/"):
		default:
			problems = append(problems, path+" depends on third-party "+dep)
		}
	}
	return problems
}

// The checker itself must catch each kind of violation.
func TestCheckerCatchesViolations(t *testing.T) {
	// An engineering rule for every Go service; the monitor has no spec requirement for it.
	domain := module + "/services/monitor/internal/domain"
	adapter := module + "/services/monitor/internal/adapters"
	pkgs := map[string]pkg{
		domain:                    {ImportPath: domain, Imports: []string{"net/http", adapter}, Deps: []string{"net/http", adapter, "github.com/jackc/pgx/v5"}},
		"net/http":                {ImportPath: "net/http", Standard: true},
		adapter:                   {ImportPath: adapter},
		"github.com/jackc/pgx/v5": {ImportPath: "github.com/jackc/pgx/v5"},
	}
	got := strings.Join(violations(pkgs, domain, nil), "\n")
	for _, want := range []string{"imports net/http", "depends on " + adapter, "third-party github.com/jackc/pgx/v5"} {
		if !strings.Contains(got, want) {
			t.Errorf("violations missing %q:\n%s", want, got)
		}
	}
}
