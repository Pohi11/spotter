# Spec Delta

## Purpose

Defines how the build walkthrough is generated from the repository and what the dashboard's "How it was built" pages show.

## ADDED Requirements

### Requirement: Every archived change is on the timeline
The walkthrough SHALL list every archived OpenSpec change in the order its archive folder entered git history, each with its title, date, the "Why" and "What Changes" sections of its proposal, the requirements its spec deltas added, modified, removed, or renamed, the decision records its proposal, design, or tasks link to, and its tasks with their state.

#### Scenario: Two archived changes
- **WHEN** the archive holds two changes
- **THEN** both are listed with their why, what changes, requirement deltas by kind, linked decision records, and tasks marked done or not

### Requirement: Each requirement links to its tests and their latest results
The walkthrough SHALL list every current requirement with its text and scenarios, every test in the Go, Python, dashboard, and browser reports that names it, each test's result and duration, and a status: `verified` when at least one linked test passed and none failed, `failing` when any linked test failed, and `untested` otherwise.

#### Scenario: Statuses
- **WHEN** one requirement has a passing Go test and a passing TypeScript test, one has a failing test, and one has none
- **THEN** their statuses are `verified` with two tests, `failing`, and `untested`

#### Scenario: Report formats
- **WHEN** reports come from `go test -json`, pytest JUnit with spec properties, and Vitest or Playwright JUnit with tags in test names
- **THEN** each test's linked requirements, result, and duration are read, and a Go test that started but never finished counts as failed

### Requirement: Decision records are shown in full
The walkthrough SHALL include every decision record in `docs/decisions/` with its number, title, date, status, and full text.

#### Scenario: One record
- **WHEN** a record titled "0001 — Use gRPC" with a date and a status exists
- **THEN** the walkthrough holds its number, title, date, status, and Markdown

### Requirement: The walkthrough reads only public sources
The generator SHALL read only `openspec/`, `docs/decisions/`, the test reports it is given, and git history, so working notes outside those paths can never reach the dashboard.

#### Scenario: Private notes
- **WHEN** the repository also holds `docs/private/notes.md` and `PLAN.md` with a marker string
- **THEN** the generated walkthrough does not contain the marker

### Requirement: The build pages show the walkthrough
The dashboard SHALL show the method, the totals (requirements verified, tests, changes, decision records), and the change timeline on `/build`; every capability and requirement with its scenarios, linked tests, and status on `/build/specs`, each requirement at a stable anchor; and every decision record on `/build/decisions`, each at an anchor by number.

#### Scenario: Timeline
- **WHEN** a visitor opens `/build`
- **THEN** every change in the walkthrough is listed, and the totals match the walkthrough's

#### Scenario: Requirement anchor
- **WHEN** a visitor follows a requirement link from a change
- **THEN** `/build/specs` shows that requirement with its linked tests and their results
