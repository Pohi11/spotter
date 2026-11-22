import { expect, test } from "@playwright/test";

import data from "../content/walkthrough.json" with { type: "json" };
import { spec } from "../lib/spec";
import { type Walkthrough, requirementAnchor, totalTests } from "../lib/walkthrough";

// The "How it was built" pages against the committed walkthrough (spec: build-walkthrough).

const w = data as unknown as Walkthrough;
const REQ = spec("build-walkthrough", "The build pages show the walkthrough");

test(`the timeline lists every change and the totals ${REQ}`, async ({ page }) => {
  await page.goto("/build");
  const changes = page.getByRole("list", { name: "Changes" }).locator(":scope > li");
  await expect(changes).toHaveCount(w.changes.length);
  for (const c of w.changes) await expect(page.locator(`[data-change="${c.slug}"]`)).toContainText(c.title);
  const totals = page.getByTestId("totals");
  await expect(totals).toContainText(`${w.totals.verified} of ${w.totals.requirements}`);
  await expect(totals).toContainText(totalTests(w).toLocaleString("en-US"));
});

test(`a requirement link opens the requirement with its tests ${REQ}`, async ({ page }) => {
  const change = w.changes.find((c) => c.slug === "phase-10-dashboard")!;
  const ref = change.requirements.find((r) => r.name === "Results come from dated reports")!;
  await page.goto("/build");
  const card = page.locator(`[data-change="${change.slug}"]`);
  await card.locator("summary").first().click();
  await card.getByRole("link", { name: ref.name }).click();
  await expect(page).toHaveURL(new RegExp(`#${requirementAnchor(ref.capability, ref.name)}$`));
  const req = page.locator(`[id="${requirementAnchor(ref.capability, ref.name)}"]`);
  await expect(req).toBeInViewport();
  const current = w.specs.find((c) => c.capability === ref.capability)!.requirements.find((r) => r.name === ref.name)!;
  await expect(req.getByRole("list", { name: `Tests for ${ref.name}` }).locator("li")).toHaveCount(current.tests.length);
  await expect(req).toHaveAttribute("data-status", current.status);
});

test(`each decision record is shown at its anchor ${REQ}`, async ({ page }) => {
  await page.goto("/build/decisions#adr-0016");
  const record = page.locator("#adr-0016");
  await expect(record).toBeInViewport();
  await record.locator("summary").click();
  await expect(record).toContainText("server-sent events");
  await expect(page.locator('li[id^="adr-"]')).toHaveCount(w.decisions.length);
});
