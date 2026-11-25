# 0022 — A guided demo driven from the browser through the public API, with a labeled recording as fallback

- **Date:** 2026-11-25
- **Status:** Accepted

## Context

The platform's main loop (traffic shifts, drift detected, a version retrained and shadowed, gates, a person's approval, the new version serving) can be seen only by running a load generator in a terminal and reading several pages at once. The dashboard should be able to show the loop on its own, step by step, with the evidence for each step, including why the monitor's PSI is what it is. The dashboard may also be hosted where no platform is running, and it must not show anything as live that isn't.

## Options considered

For sending scenario traffic:

1. **A command-line sender** (`spotterctl` or a script). Simple, but the audience sees a terminal, and the dashboard still can't show the loop by itself.
2. **A demo endpoint in the gateway** that runs a scenario server-side. Survives a closed tab, but adds demo-only behavior to a production service and a new route to the proxy's allowlist.
3. **The dashboard's server runs the scenario.** Keeps the browser simple, but makes the dashboard stateful and gives it a second, hidden path to the gateway.
4. **The browser sends tiles through the existing inference API and proxy.** Uses only the public API, exactly like any client; everything the page shows comes from the event feed and the registry. The tab must stay open while a step sends.

For explaining drift: show only the published PSI; let the dashboard fetch each version's reference and recompute; or publish the bins with each comparison.

For a host without a platform: show nothing live; or replay a recorded run.

## Decision

Option 4. The monitor publishes the bin edges and the reference and window shares behind each PSI (additive protobuf fields), so the page shows the arithmetic the monitor actually did instead of recomputing it. When the gateway can't be reached, the page replays a recording of a live run of the same page, labeled as a recording with its date, with long waits shortened and their real length stated. Approval stays a person's action in both modes.

## Consequences

- **Good:** No service or proxy rule exists only for the demo; the demo exercises the same paths as real clients.
- **Good:** The drift panel can't disagree with the monitor; it shows the monitor's own shares.
- **Good:** A static host still shows the whole loop, truthfully labeled, because the replay runs the same reducer on recorded inputs.
- **Cost:** Closing the tab stops the scenario mid-step; reopening starts again from the platform's current state.
- **Cost:** The demo's traffic enters the same monitor window as any other traffic, so on a busy platform the steps take longer.
- **Cost:** A recording shows the platform as it was on its date and has to be re-recorded when the page or the events change shape.
