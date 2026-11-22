# 0017 — TypeScript tests name their requirement in the test name

- **Date:** 2026-11-22
- **Status:** Accepted. Extends [0006](0006-go-spec-test-traceability.md).

## Context

Go tests name their requirement with `spectest.Verifies` (0006) and Python tests with a pytest marker that becomes a JUnit property. The dashboard adds TypeScript tests in Vitest and Playwright. Both write JUnit XML, but neither lets a test add a JUnit property from inside the test.

## Options considered

1. **A custom reporter per runner** that writes properties. Two reporters to maintain, tied to each runner's internal APIs.
2. **A tag in the test name:** a helper appends `[spec: <capability>: <requirement>]` to the title. Both runners copy the title into the JUnit `name` attribute unchanged.
3. **Annotations** (Playwright supports them; Vitest has no equivalent in its JUnit output).

## Decision

Option 2. `dashboard/lib/spec.ts` exports `spec(capability, requirement)`, which validates its arguments and returns the tag; tests are titled `` `${description} ${spec("dashboard", "…")}` ``. A test may carry more than one tag. `speccheck` reads tags from JUnit test names as well as from properties.

## Consequences

- **Good:** No custom reporters; the link sits in the standard report next to the result, as in Go and Python.
- **Cost:** Test titles get long. Requirement renames break links, which speccheck reports in both directions as it does for the other languages.
