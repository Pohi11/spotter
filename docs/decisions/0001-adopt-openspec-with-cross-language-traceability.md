# 0001 — Adopt OpenSpec, with spec-to-test traceability in every language

- **Date:** 2026-11-22
- **Status:** Accepted

## Context

Spotter is built to be inspected. A reviewer should be able to see, for any behavior, the requirement that asked for it, the design that shaped it, the decision records behind it, and the tests that prove it, with their latest results.

Transparency has two separate audiences:

1. **Runtime:** what happened to a specific inference job or model version (traces, events, gate results, approvals). The operations dashboard covers this.
2. **Build time:** how each feature was specified, decided, and verified. A generated build walkthrough covers this.

Spotter's tests are written in three languages: Go (registry, gateway, monitor), Python (inference worker, trainer), and TypeScript (dashboard, end-to-end tests). A traceability scheme that only works for one of them leaves most requirements unlinked.

Some contracts are already machine-checked outside the specs: protobuf files define the exact shape of every API message and event, and `buf breaking` guards them.

## Options considered

1. **No spec tool.** A build history written by hand. Simple, but not tied to code or tests, so it drifts.
2. **A home-grown spec format.** Full control, but it rebuilds an existing tool and is harder for a reviewer to follow.
3. **OpenSpec.** Each change has a proposal, spec deltas (requirements with WHEN/THEN scenarios), a design, and tasks. Finished changes are archived with a date and merged into the current specs.

## Decision

Option 3, with these rules:

- **Every phase is one OpenSpec change,** reviewed before implementation and archived when done. Phase 0 is setup only and is recorded in decision records instead.
- **Every requirement has at least one WHEN/THEN scenario that a test can pass or fail.** Scenarios name observable effects ("exactly one result is recorded for the job id"), not intentions ("the system recovers").
- **Every test that verifies a requirement names it** as `<capability>: <requirement title>`, in a form that ends up in that language's machine-readable test report:

  | Language | Mechanism | Report |
  |---|---|---|
  | Python | registered pytest marker `@pytest.mark.spec(capability, requirement)`, copied into JUnit XML properties | JUnit XML |
  | TypeScript | Playwright/Vitest test annotation `{ type: "spec", description: "<capability>: <requirement>" }` | JSON or JUnit report |
  | Go | chosen with the first Go spec in Phase 1, in its own decision record (Go has no test markers) | `go test -json` |

- **A coverage check fails the build** if any requirement has no passing linked test, or if any test names a requirement that doesn't exist.
- **Specs reference `.proto` files for message shapes** instead of repeating field lists. Requirements describe behavior; protobuf describes structure.
- **Runtime and build-time transparency are separate pages.**

Each kind of information lives in one place:

| Information | Lives in |
|---|---|
| Current system behavior, and the plan for each change | `openspec/` |
| Message and API shapes | `proto/` |
| Decisions that span more than one change | `docs/decisions/` (a change's `design.md` links here) |

## Consequences

- **Good:** The build walkthrough is generated from repository data (archived changes, specs, three test reports), so it can't drift from reality.
- **Good:** Every requirement is traceable from spec to test to latest result, whichever language the test is written in.
- **Cost:** Three report formats to parse and join, instead of one.
- **Cost:** Requirement titles become identifiers. Renaming one means updating every test that names it. The coverage check catches drift but not a test that names the wrong requirement; review is the only guard there.
- **Cost:** Some scenarios (redelivery, duplicate events, crash recovery) can only be verified against a real message bus and database. Those tests are slower and need container tooling in CI.
- **Cost:** An extra tool to install (a global npm package). Its anonymous telemetry is turned off.
