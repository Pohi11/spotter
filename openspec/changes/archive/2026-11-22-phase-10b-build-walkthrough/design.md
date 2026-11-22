# Design

## Context

Requirements live in `openspec/specs/`, the history of how they got there in `openspec/changes/archive/`, and the reasoning in `docs/decisions/`. Every test names the requirement it verifies, in a form that reaches its language's standard report ([0006](../../../../docs/decisions/0006-go-spec-test-traceability.md), [0017](../../../../docs/decisions/0017-typescript-spec-test-traceability.md)). `speccheck` already joins the two to fail CI on gaps. The walkthrough joins them again, keeps everything, and publishes the result.

## Decisions

### D1. One reader for every report format
`internal/testreport` returns every test case with its ID, result, duration, and named requirements, from `go test -json` and from JUnit XML (pytest properties, and Vitest or Playwright tags in names). `speccheck` and `tools/walkthrough` both use it, so a format is parsed in one place.

### D2. Generated JSON, committed
`go run ./tools/walkthrough` writes `dashboard/content/walkthrough.json`, which the pages import at build time. It is regenerated after the test suites run and committed at the end of each change, so the deployed dashboard shows the latest recorded run. Nothing in it is typed by hand.

### D3. Change order from git
Several changes are archived on the same day, so the folder's date can't order them. The order is the commit that first added each archive folder, as `git log --diff-filter=A` reports.

### D4. Only public paths
The generator opens `openspec/`, `docs/decisions/`, and the report files it is given, and runs `git log` scoped to archive folders. Working notes kept elsewhere in the checkout are never read.

### D5. Pages
`/build` has the method, the totals, and a timeline of changes, each expandable to its requirements (linked to `/build/specs`), decisions (linked to `/build/decisions`), and tasks. `/build/specs` lists capabilities and requirements with their scenarios and linked tests. Anchors are slugs of capability and requirement names, as in the earlier PawDesk walkthrough.
