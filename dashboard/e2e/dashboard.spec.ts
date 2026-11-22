import { type Page, expect, test } from "@playwright/test";

import fixtures from "./fixtures/gateway.json" with { type: "json" };
import worked from "../content/worked-example.json" with { type: "json" };
import { spec } from "../lib/spec";

// The dashboard against the stub gateway (design D7). Each test resets the stub's state.

const STUB = `http://localhost:${process.env.SPOTTER_E2E_STUB_PORT ?? 8099}`;
const stub = (path: string, body: object = {}) =>
  fetch(`${STUB}${path}`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body) });

test.beforeEach(async () => {
  await stub("/__reset");
});

const D = (requirement: string) => spec("dashboard", requirement);

test(`the home page follows the recorded tile ${D("The home page explains the platform with one recorded image")}`, async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1 })).toContainText("satellite-image classifier");
  const example = page.locator("#example");
  await expect(example.getByRole("img", { name: "The forest tile from the recorded run" })).toBeVisible();
  await expect(example).toContainText(worked.job.jobId);
  await expect(example).toContainText(`Forest, ${(worked.job.prediction.confidence * 100).toFixed(1)}% confident`);
  await expect(example).toContainText(`version ${worked.job.prediction.modelVersion}`);
  await expect(example.getByText("Worker runs the model")).toBeVisible();
  const classify = worked.spans.find((s) => s.name === "classify")!;
  await expect(example).toContainText(`The model itself ran for ${classify.duration_ms.toFixed(1)} ms`);
});

test(`a sample tile is classified ${D("A visitor can classify a sample tile")}`, async ({ page }) => {
  await page.goto("/#try");
  const panel = page.locator("#try");
  await panel.getByRole("radio", { name: "Forest" }).check({ force: true });
  await panel.getByRole("button", { name: "Classify this tile" }).click();
  const job = fixtures.job.job;
  await expect(panel).toContainText(job.jobId);
  await expect(panel).toContainText("Status: succeeded");
  await expect(panel).toContainText(`Forest ${(job.prediction!.confidence * 100).toFixed(1)}%`);
  await expect(panel).toContainText("Matches the true class.");
  await expect(panel).toContainText(`Served by version ${job.prediction!.modelVersion}.`);
  await expect(panel).toContainText("True class: Forest");
});

test(`an unreachable platform is reported and the panel stays usable ${D("A visitor can classify a sample tile")}`, async ({ page }) => {
  await page.route("**/api/v1/models/*/inferences", (r) => r.fulfill({ status: 502, contentType: "application/json", body: '{"message":"x"}' }));
  await page.goto("/#try");
  const panel = page.locator("#try");
  await panel.getByRole("button", { name: "Classify this tile" }).click();
  await expect(panel.getByRole("alert")).toContainText("can't be reached");
  await expect(panel.getByRole("button", { name: "Classify this tile" })).toBeEnabled();
});

async function inferenceCount(page: Page): Promise<number> {
  return Number(await page.getByTestId("count-INFERENCE").textContent());
}

test(`new events appear first and are counted ${D("The live page shows events as they arrive")}`, async ({ page }) => {
  await page.goto("/live");
  await expect(page.getByRole("status")).toContainText("Live");
  const list = page.getByRole("list", { name: "Events, newest first" });
  await expect(list.locator("li").first()).toBeVisible();
  const before = await inferenceCount(page);
  await stub("/__emit", {
    stream: "INFERENCE",
    subject: "inference.completed.eurosat-classifier",
    type: "spotter.v1.InferenceCompleted",
    payload: { jobId: "01a2aaaa-0000-7000-8000-00000e2e0001", modelName: "eurosat-classifier", prediction: { label: "River", confidence: 0.91, modelVersion: 4 } },
  });
  await expect(list.locator("li").first()).toContainText("Job completed");
  await expect(list.locator("li").first()).toContainText("job …0e2e0001: River at 91.0% by version 4");
  await expect.poll(() => inferenceCount(page)).toBe(before + 1);
});

test(`a lost connection shows reconnecting ${D("The live page shows events as they arrive")}`, async ({ page }) => {
  await page.goto("/live");
  await expect(page.getByRole("status")).toContainText("Live");
  await stub("/__drop");
  await expect(page.getByRole("status")).toContainText(/reconnecting|Live/);
  // EventSource retries within seconds and the stub accepts it again.
  await expect(page.getByRole("status")).toContainText("Live", { timeout: 20_000 });
});

test(`every version is listed with its stage ${D("The model page shows every version and its stage")}`, async ({ page }) => {
  await page.goto("/model");
  for (const v of fixtures.versions.versions) {
    const row = page.locator(`tr[data-version="${v.version}"]`);
    await expect(row).toBeVisible();
  }
  await expect(page.locator('tr[data-version="3"]')).toHaveAttribute("data-stage", "production");
  await expect(page.locator('tr[data-version="4"]')).toHaveAttribute("data-stage", "shadow");
  await page.locator('tr[data-version="4"] summary').click();
  await expect(page.locator('tr[data-version="4"]')).toContainText("shadow after retraining");
});

test(`a failed gate disables promotion ${D("Promotion is shown gate by gate and needs a person")}`, async ({ page }) => {
  await stub("/__gate", { version: 4, name: "shadow_agreement", passed: false, detail: "120 of at least 200 comparisons, agreement 100.0% (need 95%)" });
  await page.goto("/model");
  const gate = page.locator('[data-gate="shadow_agreement"]');
  await expect(gate).toHaveAttribute("data-passed", "false");
  await expect(gate).toContainText("120 of at least 200");
  await expect(page.getByRole("button", { name: "Promote version 4" })).toBeDisabled();
});

test(`approval then promotion puts the version in production ${D("Promotion is shown gate by gate and needs a person")}`, async ({ page }) => {
  await page.goto("/model");
  await expect(page.getByRole("button", { name: "Promote version 4" })).toBeDisabled();
  await page.getByLabel("Your name").fill("Test Operator");
  await page.getByLabel("Reason").fill("Gates pass and degraded accuracy improved");
  await page.getByRole("button", { name: "Approve version 4" }).click();
  await expect(page.locator('[data-gate="approval"]')).toHaveAttribute("data-passed", "true");
  await page.getByRole("button", { name: "Promote version 4" }).click();
  await expect(page.locator('tr[data-version="4"]')).toHaveAttribute("data-stage", "production");
  await expect(page.locator('tr[data-version="3"]')).toHaveAttribute("data-stage", "retired");
});

test(`rollback names the version it restores and asks first ${D("Rollback names what it will restore and asks first")}`, async ({ page }) => {
  await page.goto("/model");
  // In the recorded state version 3 is in production and version 1 was last retired.
  await page.getByRole("button", { name: "Roll back to version 1" }).click();
  await expect(page.getByText("Roll back from version 3 to version 1?")).toBeVisible();
  await expect(page.locator('tr[data-version="3"]')).toHaveAttribute("data-stage", "production"); // nothing yet
  await page.getByLabel("Reason").last().fill("e2e rollback");
  await page.getByRole("button", { name: "Confirm rollback" }).click();
  await expect(page.locator('tr[data-version="1"]')).toHaveAttribute("data-stage", "production");
  await expect(page.locator('tr[data-version="3"]')).toHaveAttribute("data-stage", "rolled back");
});

test(`a drifting version is marked and charted ${D("The drift page charts PSI over time")}`, async ({ page }) => {
  await stub("/__emit", {
    stream: "MONITOR",
    subject: "monitor.drift.evaluated.eurosat-classifier",
    type: "spotter.v1.DriftEvaluated",
    payload: {
      modelName: "eurosat-classifier",
      versionId: "e2e-version-9",
      version: 9,
      windowSize: 200,
      drifting: true,
      evaluatedAt: new Date().toISOString(),
      features: ["mean_r", "mean_g", "mean_b", "contrast", "confidence"].map((f) => ({ feature: f, psi: f === "mean_g" ? 0.8 : 0.03 })),
    },
  });
  await page.goto("/drift");
  const fig = page.locator('figure[data-version="9"]');
  await expect(fig).toHaveAttribute("data-drifting", "true");
  await expect(fig).toContainText("drifting");
  await expect(fig.locator(".recharts-line")).toHaveCount(5);
  await expect(fig).toContainText("0.800");
});

test(`results come from the dated reports ${D("Results come from dated reports")}`, async ({ page }) => {
  const report = (await import("../content/benchmarks/2026-11-22-model-variants.json", { with: { type: "json" } })).default;
  await page.goto("/results");
  for (const v of report.variants) {
    const row = page.locator(`tr[data-variant="${v.name}"]`);
    await expect(row).toContainText(`${(v.accuracy * 100).toFixed(2)}%`);
    await expect(row).toContainText(`${Math.round(v.size_bytes / 1024).toLocaleString()} KB`);
  }
  await expect(page.locator("#variants")).toContainText(`Measured ${report.date}`);
});

test(`every diagram has a title and a description ${D("Architecture diagrams have text equivalents")}`, async ({ page }) => {
  await page.goto("/architecture");
  const diagrams = page.locator('svg[role="img"]');
  expect(await diagrams.count()).toBeGreaterThanOrEqual(6);
  for (const svg of await diagrams.all()) {
    const title = await svg.getAttribute("aria-labelledby");
    const desc = await svg.getAttribute("aria-describedby");
    await expect(page.locator(`[id="${title}"]`)).not.toBeEmpty();
    expect((await page.locator(`[id="${desc}"]`).textContent())?.length ?? 0).toBeGreaterThan(80);
  }
  await expect(page.locator("#parts")).toContainText("services/gateway");
});

const PAGES = ["/", "/live", "/model", "/drift", "/results", "/architecture", "/build", "/build/specs", "/build/decisions"];

test(`no page scrolls sideways at phone width ${D("Pages work by keyboard and at phone width")}`, async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 });
  for (const path of PAGES) {
    await page.goto(path);
    // Live pages keep an event stream open, so wait for content rather than an idle network.
    await expect(page.locator("main h1")).toBeVisible();
    await page.waitForTimeout(500);
    const [scroll, inner, wide] = await page.evaluate(() => {
      // What sticks out, ignoring anything inside its own horizontal scroller.
      const wide = [...document.querySelectorAll("body *")]
        .filter((e) => e.getBoundingClientRect().right > window.innerWidth + 0.5)
        .filter((e) => {
          for (let a = e.parentElement; a; a = a.parentElement) if (getComputedStyle(a).overflowX !== "visible") return false;
          return true;
        })
        .slice(0, 3)
        .map((e) => `${e.tagName} "${(e.textContent ?? "").slice(0, 40)}"`);
      const sw = document.documentElement.scrollWidth;
      const edge = [...document.querySelectorAll("body *")]
        .filter((e) => Math.abs(e.getBoundingClientRect().right - sw) < 2)
        .slice(0, 3)
        .map((e) => `${e.tagName}.${String(e.getAttribute("class") ?? "").slice(0, 50)}`);
      return [sw, window.innerWidth, `${wide.join("; ")} | at the edge: ${edge.join("; ")}`] as const;
    });
    expect(scroll, `${path}: ${wide}`).toBeLessThanOrEqual(inner);
  }
});

test(`keyboard focus is visible ${D("Pages work by keyboard and at phone width")}`, async ({ page }) => {
  for (const path of PAGES) {
    await page.goto(path);
    await page.keyboard.press("Tab"); // the skip link
    await page.keyboard.press("Tab");
    const outline = await page.evaluate(() => {
      const el = document.activeElement as HTMLElement | null;
      return el && el !== document.body ? getComputedStyle(el).outlineStyle : "none";
    });
    expect(outline, path).not.toBe("none");
  }
});
