package testreport_test

import (
	"strings"
	"testing"

	"github.com/Pohi11/spotter/internal/spectest"
	"github.com/Pohi11/spotter/internal/testreport"
)

const capability = "build-walkthrough"

const goJSON = `{"Action":"run","Package":"p","Test":"TestA"}
{"Action":"output","Package":"p","Test":"TestA","Output":"    a_test.go:9: spec: model-lifecycle: Only listed stage transitions are allowed\n"}
{"Action":"pass","Package":"p","Test":"TestA","Elapsed":0.5}
{"Action":"run","Package":"p","Test":"TestB/sub"}
{"Action":"fail","Package":"p","Test":"TestB/sub","Elapsed":0.1}
{"Action":"run","Package":"p","Test":"TestC"}
{"Action":"skip","Package":"p","Test":"TestC"}
{"Action":"run","Package":"p","Test":"TestD"}
{"Action":"pass","Package":"p","Elapsed":1}
`

func TestReadGo(t *testing.T) {
	spectest.Verifies(t, capability, "Each requirement links to its tests and their latest results")
	cases, err := testreport.ReadGo(strings.NewReader(goJSON))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, c := range cases {
		got[c.ID] = c.Status
	}
	want := map[string]string{"p TestA": "passed", "p TestB/sub": "failed", "p TestC": "skipped", "p TestD": "failed"}
	for id, s := range want {
		if got[id] != s {
			t.Errorf("%s = %q, want %q (TestD never finished, so it counts as failed)", id, got[id], s)
		}
	}
	if len(cases[0].Requirements) != 1 || cases[0].Requirements[0] != "model-lifecycle: Only listed stage transitions are allowed" {
		t.Errorf("requirements %v", cases[0].Requirements)
	}
}

const junit = `<?xml version="1.0"?>
<testsuites><testsuite name="s" timestamp="2026-11-22T20:00:00">
  <testcase classname="a.test.ts" name="forwards routes [spec: dashboard: The browser reaches the gateway only through an allowlisted proxy]" time="0.02"/>
  <testcase classname="tests.test_x" name="test_y"><properties><property name="spec" value="training-data: Splits are deterministic"/></properties><failure/></testcase>
  <testsuite name="nested" timestamp="2026-11-22T21:00:00"><testcase classname="e2e" name="skipped one"><skipped/></testcase></testsuite>
</testsuite></testsuites>`

func TestReadJUnit(t *testing.T) {
	spectest.Verifies(t, capability, "Each requirement links to its tests and their latest results")
	cases, latest, err := testreport.ReadJUnit(strings.NewReader(junit))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 3 || latest != "2026-11-22T21:00:00" {
		t.Fatalf("cases %+v latest %q", cases, latest)
	}
	if cases[0].ID != "a.test.ts forwards routes" || cases[0].Status != "passed" || cases[0].Seconds != 0.02 ||
		cases[0].Requirements[0] != "dashboard: The browser reaches the gateway only through an allowlisted proxy" {
		t.Errorf("tagged case %+v", cases[0])
	}
	if cases[1].Status != "failed" || cases[1].Requirements[0] != "training-data: Splits are deterministic" {
		t.Errorf("property case %+v", cases[1])
	}
	if cases[2].Status != "skipped" {
		t.Errorf("nested case %+v", cases[2])
	}
}
