# 0006 — Go tests name their requirement with `spectest.Verifies`

- **Date:** 2026-11-22
- **Status:** Accepted

## Context

Decision record [0001](0001-adopt-openspec-with-cross-language-traceability.md) requires every test that verifies a requirement to name it as `<capability>: <requirement title>`, in a form that reaches the test report, and deferred the Go mechanism to the first Go spec. Go's `testing` package has no markers or annotations. The standard machine-readable report is `go test -json`, a stream of events per test: start, output lines, and pass or fail.

## Options considered

1. **Naming convention** (`TestModelLifecycle_OnlyListedTransitions`). No helper needed, but requirement titles have spaces and punctuation, renames silently break the link, and a test can't name two requirements.
2. **Structured comments** (`// spec: model-lifecycle: …`) parsed from source. Not in the test report, so a parser has to match comments to test functions and can't tell whether the test ran or passed.
3. **A helper call** at the start of the test that logs a structured line. The line lands in the `go test -json` output event of exactly that test (or subtest), next to its pass/fail event. A test can call it more than once.

## Decision

Option 3.

- `spectest.Verifies(t, capability, requirement)` validates its arguments (capability is a lowercase slug, requirement is non-blank) and logs `spec: <capability>: <requirement>`. A malformed call fails the test.
- `tools/speccheck` reads the requirements from `openspec/specs/` and from active changes in `openspec/changes/` (archived changes excluded), and joins them with a `go test -json` report. It fails if any requirement has no passing linked test, or if any test names a requirement that doesn't exist.
- CI runs the Go tests through gotestsum, which prints readable output and writes the JSON report in one run.

## Consequences

- **Good:** The link sits in the standard report beside the result; no custom test runner.
- **Good:** Works for subtests: table-driven tests can link each case.
- **Cost:** Requirement titles become identifiers; renaming one means updating the calls. speccheck catches drift in both directions.
- **Cost:** A test can name a requirement it doesn't really verify. Review is the only guard.
- **Later:** speccheck will also read pytest JUnit XML and the TypeScript report when those languages get specs.
