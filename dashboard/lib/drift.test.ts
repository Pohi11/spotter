import { expect, test } from "vitest";

import { byVersion, evaluations } from "./drift";
import { spec } from "./spec";
import type { FeedEvent } from "./types";

const REQ = spec("dashboard", "The drift page charts PSI over time");

const evaluated = (version: number, at: string, drifting: boolean, meanR: number): FeedEvent => ({
  stream: "MONITOR",
  subject: "monitor.drift.evaluated.eurosat-classifier",
  sequence: 1,
  time: at,
  type: "spotter.v1.DriftEvaluated",
  payload: {
    versionId: `v${version}`,
    version,
    drifting,
    windowSize: 200,
    evaluatedAt: at,
    features: [
      { feature: "mean_r", psi: meanR },
      { feature: "contrast", psi: 0.03 },
    ],
  },
});

test(`series are per version, oldest first, with the latest state ${REQ}`, () => {
  const events = [
    evaluated(3, "2026-11-22T22:00:10Z", true, 0.8),
    evaluated(3, "2026-11-22T22:00:00Z", false, 0.05),
    evaluated(2, "2026-11-22T22:00:05Z", false, 0.02),
    { ...evaluated(1, "2026-11-22T22:00:00Z", false, 0), type: "spotter.v1.DriftDetected" },
  ];
  const series = byVersion(evaluations(events));
  expect(series.map((s) => [s.version, s.drifting])).toEqual([
    [3, true],
    [2, false],
  ]);
  expect(series[0].points.map((p) => p.mean_r)).toEqual([0.05, 0.8]);
  expect(series[0].points[1].mean_r).toBeGreaterThan(0.2);
});
