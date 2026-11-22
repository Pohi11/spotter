import { expect, test } from "vitest";

import { addEvent, countByStream, summarize } from "./events";
import { spec } from "./spec";
import type { FeedEvent } from "./types";

const REQ = spec("dashboard", "The live page shows events as they arrive");

const ev = (over: Partial<FeedEvent>): FeedEvent => ({
  stream: "INFERENCE",
  subject: "inference.completed.eurosat-classifier",
  sequence: 1,
  time: "2026-11-22T22:18:32.692Z",
  type: "spotter.v1.InferenceCompleted",
  payload: null,
  ...over,
});

const completed = ev({
  payload: {
    jobId: "01a20556-ea13-7499-ae28-97c9014de26a",
    modelName: "eurosat-classifier",
    prediction: { label: "Forest", confidence: 0.9972019, modelVersion: 3 },
  },
});

test(`a completion names the job, class, confidence, and version ${REQ}`, () => {
  expect(summarize(completed)).toEqual({ title: "Job completed", detail: "job …014de26a: Forest at 99.7% by version 3", tone: "model" });
});

test(`a stage change names the version, stages, and reason ${REQ}`, () => {
  const s = summarize(
    ev({
      stream: "REGISTRY",
      type: "spotter.v1.ModelVersionStageChanged",
      payload: { version: 2, fromStage: "MODEL_VERSION_STAGE_PRODUCTION", toStage: "MODEL_VERSION_STAGE_ROLLED_BACK", reason: "rollback drill" },
    }),
  );
  expect(s).toEqual({ title: "Stage changed", detail: "version 2: production to rolled back, “rollback drill”", tone: "fail" });
});

test(`drift events name the drifting features ${REQ}`, () => {
  const features = [
    { feature: "mean_r", psi: 0.81 },
    { feature: "contrast", psi: 0.05 },
  ];
  expect(summarize(ev({ stream: "MONITOR", type: "spotter.v1.DriftDetected", payload: { version: 3, features } })).detail).toBe(
    "version 3: mean_r PSI 0.81, contrast PSI 0.05",
  );
  expect(
    summarize(ev({ stream: "MONITOR", type: "spotter.v1.DriftEvaluated", payload: { version: 3, drifting: true, features } })).detail,
  ).toBe("version 3: drifting on mean_r PSI 0.81");
});

test(`an unknown type still shows its subject ${REQ}`, () => {
  expect(summarize(ev({ type: "", subject: "registry.events.new_thing" }))).toEqual({
    title: "Unknown event",
    detail: "registry.events.new_thing",
    tone: "plain",
  });
});

test(`new events go first, repeats are ignored, and streams are counted ${REQ}`, () => {
  const a = ev({ sequence: 1, time: "2026-11-22T22:00:00Z" });
  const b = ev({ sequence: 2, time: "2026-11-22T22:00:01Z" });
  const c = ev({ stream: "MONITOR", sequence: 1, time: "2026-11-22T22:00:02Z" });
  let list = addEvent([], a);
  list = addEvent(list, b);
  list = addEvent(list, c);
  list = addEvent(list, b); // replayed after a reconnect
  expect(list.map((e) => `${e.stream}:${e.sequence}`)).toEqual(["MONITOR:1", "INFERENCE:2", "INFERENCE:1"]);
  expect(countByStream(list)).toEqual({ INFERENCE: 2, REGISTRY: 0, MONITOR: 1 });
});
