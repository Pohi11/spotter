# Proposal

## Why

The method behind Spotter (a spec per change, a decision record per lasting choice, a test per requirement, measured numbers only) is only visible to someone who reads the repository. A reader evaluating the work needs to see every change, every requirement with the tests that prove it and their latest result, and every decision record, without trusting a hand-written summary that can drift from the code.

## What Changes

- `tools/walkthrough` (Go) generates `dashboard/content/walkthrough.json` from the archived OpenSpec changes, the current specs, the decision records, git history (for change order), and the latest Go, Python, Vitest, and Playwright reports.
- `internal/testreport` reads every report format once; `speccheck` and the walkthrough share it.
- The dashboard's "How it was built" pages: `/build` (method, totals, and the change timeline), `/build/specs` (each capability and requirement with its scenarios, linked tests, and status), and `/build/decisions` (every record in full).

## Capabilities

### New Capabilities
- `build-walkthrough`: how the build walkthrough is generated and what it shows.

### Modified Capabilities
None.

## Impact

- **New:** `internal/testreport`, `tools/walkthrough`, three dashboard pages, a committed `walkthrough.json`.
- **Changed:** `speccheck` reads reports through `internal/testreport`; its behavior is unchanged.
- **Out of scope:** checking coverage (that stays in `speccheck`).
