import { expect, test } from "vitest";

import { binLabel, breakdown, worst } from "./psi";
import { spec } from "./spec";
import type { FeatureDrift } from "./types";

const REQ = spec("dashboard", "The demo page shows the arithmetic behind each drift evaluation");

// Shares from a real DriftEvaluated (hazed traffic, mean_b on version 3).
const hazed: FeatureDrift = {
  feature: "mean_b",
  psi: 0,
  ksStatistic: 0.6,
  ksPValue: 0,
  referenceMean: 0.35,
  windowMean: 0.52,
  binEdges: [0.22, 0.25, 0.27, 0.29, 0.31, 0.33, 0.36, 0.4, 0.46],
  referenceShares: [0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1],
  windowShares: [0.03, 0.04, 0.05, 0.05, 0.06, 0.06, 0.07, 0.09, 0.1, 0.45],
};
hazed.psi = hazed.referenceShares!.reduce((s, r, i) => s + (hazed.windowShares![i] - r) * Math.log(hazed.windowShares![i] / r), 0);

test(`contributions add up to the published PSI ${REQ}`, () => {
  const b = breakdown(hazed)!;
  expect(b.bins).toHaveLength(10);
  expect(b.sum.toFixed(3)).toBe(hazed.psi.toFixed(3));
  for (const bin of b.bins) expect(bin.contribution).toBeGreaterThanOrEqual(0);
  // The top bin, where hazed tiles pile up, contributes most.
  expect(b.bins.indexOf(b.bins.reduce((a, x) => (x.contribution > a.contribution ? x : a)))).toBe(9);
});

test(`outer bins are open-ended and labeled by their edges ${REQ}`, () => {
  const b = breakdown(hazed)!;
  expect(b.bins[0]).toMatchObject({ lo: null, hi: 0.22 });
  expect(b.bins[9]).toMatchObject({ lo: 0.46, hi: null });
  expect(binLabel(b.bins[0])).toBe("below 0.220");
  expect(binLabel(b.bins[4])).toBe("0.290 to 0.310");
  expect(binLabel(b.bins[9])).toBe("0.460 and up");
});

test(`events without bins have no breakdown ${REQ}`, () => {
  const old: FeatureDrift = { ...hazed, binEdges: undefined, referenceShares: undefined, windowShares: undefined };
  expect(breakdown(old)).toBeNull();
  expect(worst([old, { ...old, feature: "contrast", psi: 9 }])?.feature).toBe("contrast");
});
