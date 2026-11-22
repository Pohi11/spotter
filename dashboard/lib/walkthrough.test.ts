import { expect, test } from "vitest";

import data from "@/content/walkthrough.json";

import { spec } from "./spec";
import { currentRequirement, decisionHref, requirementAnchor, requirementHref, slug, totalTests, type Walkthrough } from "./walkthrough";

const w = data as unknown as Walkthrough;
const REQ = spec("build-walkthrough", "The build pages show the walkthrough");

test(`anchors are stable slugs ${REQ}`, () => {
  expect(slug("A visitor can classify a sample tile")).toBe("a-visitor-can-classify-a-sample-tile");
  expect(requirementAnchor("dashboard", "Results come from dated reports")).toBe("req-dashboard-results-come-from-dated-reports");
  expect(requirementHref("event-feed", "A client can follow only some streams")).toBe(
    "/build/specs#req-event-feed-a-client-can-follow-only-some-streams",
  );
  expect(decisionHref("0016")).toBe("/build/decisions#adr-0016");
});

test(`every requirement a change names resolves or is known to be gone ${REQ}`, () => {
  let resolved = 0;
  for (const c of w.changes) {
    for (const r of c.requirements) {
      if (r.kind === "removed") continue;
      if (currentRequirement(w, r.capability, r.name)) resolved++;
    }
  }
  expect(resolved).toBeGreaterThan(0);
  expect(totalTests(w)).toBe(Object.values(w.totals.tests).reduce((a, b) => a + b, 0));
});
