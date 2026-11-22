import { readFileSync } from "node:fs";
import { join } from "node:path";

import { expect, test } from "vitest";

import { LOAD_RUNS, REPORTS_DIR, loadResults, runKey } from "./results";
import { spec } from "./spec";

const REQ = spec("dashboard", "Results come from dated reports");

test(`every result is read from its dated report ${REQ}`, () => {
  const r = loadResults();
  const variants = JSON.parse(readFileSync(join(REPORTS_DIR, "2026-11-22-model-variants.json"), "utf-8"));
  expect(r.variants).toEqual(variants);
  expect(r.variants.variants.map((v) => v.name)).toEqual(["pytorch-fp32", "onnx-fp32", "onnx-int8"]);
  expect(r.load.map((l) => runKey(l.file))).toEqual(Object.keys(LOAD_RUNS));
  for (const l of r.load) expect(l.report.completed_per_second).toBeGreaterThan(0);
  expect(r.drift.conditions.length).toBeGreaterThan(0);
  expect(r.promotion.promote_to_serving_ms).toBeGreaterThan(0);
  expect(r.spike.autoscaled.report.completed).toBeGreaterThan(0);
});
