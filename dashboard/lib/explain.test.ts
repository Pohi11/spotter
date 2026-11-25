import { expect, test } from "vitest";

import { compact, explain } from "./explain";
import { spec } from "./spec";
import type { FeedEvent } from "./types";

const REQ = spec("dashboard", "The demo page explains every event it sees");

const ev = (type: string, subject: string, payload: Record<string, unknown>): FeedEvent => ({
  stream: "MONITOR",
  subject,
  sequence: 1,
  time: "2026-11-25T00:00:00Z",
  type: `spotter.v1.${type}`,
  payload,
});

test(`a drift event names its publisher, subject, features, and consumer ${REQ}`, () => {
  const e = explain(
    ev("DriftDetected", "monitor.drift.detected.eurosat-classifier", {
      version: 3,
      features: [{ feature: "mean_b", psi: 0.43 }, { feature: "mean_g", psi: 0.22 }],
    }),
  );
  expect(e.publisher).toBe("monitor");
  expect(e.story).toBe(true);
  expect(e.title).toBe("Drift detected on version 3");
  expect(e.why).toContain("monitor.drift.detected.eurosat-classifier");
  expect(e.why).toContain("Mean blue (PSI 0.43)");
  expect(e.why).toContain("Mean green (PSI 0.22)");
  expect(e.why).toContain("retrainer consumes");
});

test(`every event type the platform publishes has a publisher and a reason ${REQ}`, () => {
  const types: [string, string][] = [
    ["InferenceRequested", "gateway"],
    ["InferenceCompleted", "worker"],
    ["ShadowCompared", "worker"],
    ["InferenceFailed", "worker"],
    ["ModelVersionRegistered", "registry"],
    ["ModelVersionStageChanged", "registry"],
    ["DriftEvaluated", "monitor"],
    ["DriftDetected", "monitor"],
    ["DriftResolved", "monitor"],
  ];
  for (const [type, publisher] of types) {
    const e = explain(ev(type, "s", { version: 1, toStage: "MODEL_VERSION_STAGE_SHADOW", features: [], prediction: { label: "Forest" } }));
    expect(e.publisher, type).toBe(publisher);
    expect(e.why.length, type).toBeGreaterThan(20);
  }
  expect(explain(ev("Mystery", "x.y", {})).publisher).toBe("other");
});

test(`a stage change to shadow says what the worker does next ${REQ}`, () => {
  const e = explain(ev("ModelVersionStageChanged", "registry.events.model_version_stage_changed", { version: 5, fromStage: "MODEL_VERSION_STAGE_VALIDATED", toStage: "MODEL_VERSION_STAGE_SHADOW", reason: "retrained for drift" }));
  expect(e.title).toBe("Version 5: validated → shadow");
  expect(e.why).toContain("loads this version as its shadow");
  expect(e.why).toContain("retrained for drift");
});

test(`compacted completions drop only the fields the demo never reads ${REQ}`, () => {
  const full = ev("InferenceCompleted", "inference.completed.m", {
    jobId: "j",
    prediction: { label: "Forest", confidence: 0.9, modelVersion: 3, scores: [{ label: "Forest", score: 0.9 }] },
    inputStats: { channelMean: [0.3, 0.3, 0.3] },
  });
  expect(compact(full).payload).toEqual({ jobId: "j", prediction: { label: "Forest", confidence: 0.9, modelVersion: 3 } });
  const other = ev("DriftEvaluated", "s", { version: 1 });
  expect(compact(other)).toBe(other);
});
