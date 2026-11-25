import { type Page, expect, test } from "@playwright/test";

import fixtures from "./fixtures/gateway.json" with { type: "json" };
import { spec } from "../lib/spec";

// The guided demo against the stub gateway (spec: dashboard). The full loop needs the real
// platform; these check what the page itself promises.

const STUB = `http://localhost:${process.env.SPOTTER_E2E_STUB_PORT ?? 8099}`;
const stub = (path: string) => fetch(`${STUB}${path}`, { method: "POST", headers: { "content-type": "application/json" }, body: "{}" });
const D = (requirement: string) => spec("dashboard", requirement);

test.beforeEach(async () => {
  await stub("/__reset");
});

/** Every request the page sends to the gateway through the proxy. */
function gatewayRequests(page: Page): string[] {
  const seen: string[] = [];
  page.on("request", (r) => {
    const u = new URL(r.url());
    if (u.pathname.startsWith("/api/")) seen.push(`${r.method()} ${u.pathname}`);
  });
  return seen;
}

test(`sending steps submit tiles at the chosen pace ${D("The demo page walks through the loop step by step")}`, async ({ page }) => {
  await stub("/__unique-jobs");
  const seen = gatewayRequests(page);
  await page.goto("/demo");
  await expect(page.getByText("Live: connected to the running platform")).toBeVisible();
  const normal = page.locator('[data-step="normal"]');
  await normal.getByRole("combobox").selectOption("5");
  await normal.getByRole("button", { name: "Start sending clean tiles" }).click();
  await expect(normal).toHaveAttribute("data-status", "running");
  await page.waitForTimeout(2_000);
  const posts = seen.filter((s) => s.endsWith("/inferences")).length;
  expect(posts).toBeGreaterThanOrEqual(6);
  expect(posts).toBeLessThanOrEqual(14);
  await expect(page.getByTestId("progress-normal")).toContainText("clean tiles sent");
});

test(`the decision waits for a person ${D("The demo page never approves")}`, async ({ page }) => {
  const seen = gatewayRequests(page);
  await page.goto("/demo");
  await expect(page.getByText("Live: connected to the running platform")).toBeVisible();
  await page.getByLabel("Presenting a part of it? Skip to").selectOption("decide");
  await page.getByRole("button", { name: "Skip", exact: true }).click();

  const decide = page.locator('[data-step="decide"]');
  await expect(decide).toHaveAttribute("aria-current", "step");
  await expect(decide.locator('[data-gate="approval"]')).toHaveAttribute("data-passed", "false");
  await expect(decide.locator('[data-gate="shadow_agreement"]')).toHaveAttribute("data-passed", "true");
  await page.waitForTimeout(3_000); // the gates are polled; nothing else may be sent
  expect(seen.filter((s) => s.startsWith("POST"))).toEqual([]);
  await expect(decide.getByRole("button", { name: "Promote version 4" })).toBeDisabled();

  await decide.getByLabel("Your name").fill("A. Reviewer");
  await decide.getByLabel("Reason").fill("gates passed; checked degraded accuracy");
  await decide.getByRole("button", { name: "Approve version 4" }).click();
  await expect(decide.locator('[data-gate="approval"]')).toHaveAttribute("data-passed", "true");
  expect(seen.filter((s) => s.startsWith("POST"))).toEqual([`POST /api/v1/versions/${fixtures.versions.versions.find((v) => v.version === 4)!.id}:approve`]);

  await decide.getByRole("button", { name: "Promote version 4" }).click();
  await expect(decide).toHaveAttribute("data-status", "done");
  await expect(page.locator('[data-step="after"]')).toHaveAttribute("data-status", "ready");
});

test(`a static host offers the dated recording and replays it without the gateway ${D("The demo page replays a recording when the platform is unreachable")}`, async ({ page }) => {
  await stub("/__down");
  const seen = gatewayRequests(page);
  await page.goto("/demo");
  await expect(page.getByText("The platform can't be reached from this site")).toBeVisible();
  const watch = page.getByRole("button", { name: /Watch the recorded run from/ });
  await expect(watch).toBeVisible();
  const before = seen.length;
  await watch.click();
  await expect(page.getByTestId("replay-banner")).toContainText("nothing is sent to any platform");
  await page.getByLabel("Speed").selectOption("16");
  await page.getByRole("button", { name: "Play the recording" }).click();
  await expect(page.locator('[data-step="normal"]')).toHaveAttribute("data-status", "done", { timeout: 30_000 });
  await expect(page.locator('[data-step="haze"]')).toHaveAttribute("data-status", "done", { timeout: 30_000 });
  await expect(page.getByTestId("psi-sum")).toBeVisible();
  expect(seen.slice(before)).toEqual([]);
});

test(`the drift panel's contributions add up to the PSI ${D("The demo page shows the arithmetic behind each drift evaluation")}`, async ({ page }) => {
  await stub("/__down");
  await page.goto("/demo?mode=replay"); // opens the replay directly
  await expect(page.getByTestId("replay-banner")).toBeVisible();
  await page.getByLabel("Speed").selectOption("16");
  await page.getByRole("button", { name: "Play the recording" }).click();
  await expect(page.locator('[data-step="haze"]')).toHaveAttribute("data-status", "done", { timeout: 30_000 });
  await page.getByRole("button", { name: "Pause" }).click(); // hold one evaluation still while reading it
  const panel = page.locator("#drift-math-h").locator("..");
  const rows = panel.getByRole("table").locator("tbody tr");
  await expect(rows).toHaveCount(10);
  const adds = await rows.locator("td:last-child").allTextContents();
  const sum = adds.map(Number).reduce((a, b) => a + b, 0);
  const shown = Number(await page.getByTestId("psi-sum").textContent());
  expect(Math.abs(sum - shown)).toBeLessThan(0.006); // ten values rounded to 3 decimals
  const chosen = panel.getByRole("button", { pressed: true });
  await expect(chosen.locator("span").last()).toHaveText(shown.toFixed(3));
});
