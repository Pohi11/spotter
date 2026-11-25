import { describe, expect, test } from "vitest";

import { type Input, type State, type VersionInfo, WINDOW, accuracy, initialState, sending, step } from "./scenario";
import { spec } from "./spec";
import type { FeedEvent, Gate } from "./types";

const WALK = spec("dashboard", "The demo page walks through the loop step by step");
const EVIDENCE = spec("dashboard", "Demo steps complete on the platform's evidence");
const ACCURACY = spec("dashboard", "The demo page shows accuracy only for its own labeled tiles");
const NEVER = spec("dashboard", "The demo page never approves");

const V3: VersionInfo = { id: "v3", version: 3, stage: "MODEL_VERSION_STAGE_PRODUCTION", framework: "onnx", metrics: {} };
const V4: VersionInfo = { id: "v4", version: 4, stage: "MODEL_VERSION_STAGE_SHADOW", framework: "pytorch", metrics: {} };

let seq = 0;
const ev = (type: string, payload: Record<string, unknown>, stream = "INFERENCE"): Input => ({
  kind: "event",
  ev: { stream, subject: "s", sequence: ++seq, time: "2026-11-25T00:00:00Z", type: `spotter.v1.${type}`, payload } as FeedEvent,
});
const feat = (feature: string, psi: number) => ({ feature, psi, ksStatistic: 0, ksPValue: 1, referenceMean: 0.3, windowMean: 0.3 });
const evaluated = (versionId: string, version: number, drifting: boolean, psi = 0.05) =>
  ev("DriftEvaluated", { versionId, version, drifting, windowSize: 200, features: [feat("mean_r", psi)] }, "MONITOR");
const completedFor = (jobId: string, label: string, version: number) => ev("InferenceCompleted", { jobId, prediction: { label, modelVersion: version, confidence: 0.9 } });
const stage = (versionId: string, version: number, toStage: string, reason = "") =>
  ev("ModelVersionStageChanged", { versionId, version, fromStage: "MODEL_VERSION_STAGE_VALIDATED", toStage, reason }, "REGISTRY");

const run = (inputs: Input[], s: State = initialState()) => inputs.reduce(step, s);

/** n tiles sent and completed in the current step, labeled right or wrong. */
function traffic(s: State, n: number, version: number, right: (i: number) => boolean = () => true, prefix = "j"): State {
  for (let i = 0; i < n; i++) {
    const jobId = `${prefix}${s.current}${i}`;
    s = step(s, { kind: "sent", jobId, tile: `t${i}`, cls: "Forest", variant: "clean" });
    s = step(s, completedFor(jobId, right(i) ? "Forest" : "SeaLake", version));
  }
  return s;
}

const atHaze = () => {
  let s = run([{ kind: "versions", versions: [V3] }, { kind: "start", step: "normal" }]);
  s = traffic(s, WINDOW, 3);
  s = step(s, evaluated("v3", 3, false));
  return step(s, { kind: "start", step: "haze" });
};

describe("steps", () => {
  test(`a sending step sends until its evidence arrives ${WALK}`, () => {
    let s = run([{ kind: "versions", versions: [V3] }]);
    expect(s.current).toBe("normal");
    expect(sending(s)).toBeNull(); // waits for a click
    s = step(s, { kind: "start", step: "normal" });
    expect(sending(s)).toEqual({ variant: "clean", remaining: 600 });
    s = traffic(s, 10, 3);
    expect(s.steps.normal).toMatchObject({ sent: 10, completed: 10, status: "running" });
    expect(sending(s)?.remaining).toBe(590);
  });

  test(`normal traffic ends on a clean evaluation after a full window ${EVIDENCE}`, () => {
    let s = run([{ kind: "versions", versions: [V3] }, { kind: "start", step: "normal" }]);
    s = traffic(s, 120, 3);
    s = step(s, evaluated("v3", 3, false));
    expect(s.current).toBe("normal"); // not 200 of its own tiles yet
    s = traffic(s, 80, 3, () => true, "k");
    s = step(s, evaluated("v3", 3, true, 0.4));
    expect(s.current).toBe("normal");
    expect(s.steps.normal.note).toContain("still drifting");
    s = step(s, evaluated("v3", 3, false));
    expect(s.current).toBe("haze");
    expect(s.steps.normal.status).toBe("done");
    expect(s.steps.haze.status).toBe("ready");
  });

  test(`drift ends the haze step ${EVIDENCE}`, () => {
    let s = atHaze();
    expect(sending(s)?.variant).toBe("haze");
    s = traffic(s, 64, 3, () => false);
    s = step(s, ev("DriftDetected", { versionId: "v9", version: 9, features: [feat("mean_b", 0.5)] }, "MONITOR"));
    expect(s.current).toBe("haze"); // another version's drift doesn't count
    s = step(s, ev("DriftDetected", { versionId: "v3", version: 3, features: [feat("mean_b", 0.43)] }, "MONITOR"));
    expect(s.current).toBe("retrain");
    expect(s.steps.haze.status).toBe("done");
    expect(s.steps.haze.result).toContain("after 64 hazed tiles");
    expect(sending(s)).toBeNull(); // the page stops sending
    expect(s.steps.retrain.status).toBe("running");
  });

  test(`a new version reaching shadow becomes the candidate ${EVIDENCE}`, () => {
    let s = atHaze();
    s = step(s, ev("DriftDetected", { versionId: "v3", version: 3, features: [] }, "MONITOR"));
    s = step(s, ev("ModelVersionRegistered", { versionId: "v5", version: 5 }, "REGISTRY"));
    s = step(s, stage("v2", 2, "MODEL_VERSION_STAGE_SHADOW")); // older than production: not a candidate
    expect(s.current).toBe("retrain");
    s = step(s, stage("v5", 5, "MODEL_VERSION_STAGE_SHADOW"));
    expect(s.candidate).toEqual({ id: "v5", version: 5 });
    expect(s.current).toBe("shadow");
    expect(s.steps.shadow.status).toBe("ready");
  });

  test(`the shadow step ends when the agreement gate passes, then waits for promotion ${EVIDENCE}`, () => {
    let s = run([{ kind: "versions", versions: [V3, V4] }, { kind: "start", step: "shadow", candidateId: "v4" }]);
    const gates = (passed: boolean): Gate[] => [
      { name: "shadow_agreement", passed, detail: passed ? "594 of at least 200 comparisons, agreement 98.5% (need 95%)" : "12 of at least 200 comparisons" },
      { name: "approval", passed: false, detail: "0 approval(s) recorded" },
    ];
    s = step(s, { kind: "gates", versionId: "v4", gates: gates(false) });
    expect(s.current).toBe("shadow");
    s = step(s, { kind: "gates", versionId: "v4", gates: gates(true) });
    expect(s.current).toBe("decide");
    expect(s.steps.decide.status).toBe("running");
    s = step(s, stage("v4", 4, "MODEL_VERSION_STAGE_PRODUCTION", "promoted after review"));
    expect(s.current).toBe("after");
    expect(s.steps.decide.result).toContain("promoted after review");
  });

  test(`skipping ahead marks earlier steps skipped ${WALK}`, () => {
    const s = run([{ kind: "versions", versions: [V3, V4] }, { kind: "start", step: "decide", candidateId: "v4" }]);
    expect(s.steps.normal.status).toBe("skipped");
    expect(s.steps.shadow.status).toBe("skipped");
    expect(s.current).toBe("decide");
    expect(s.candidate?.version).toBe(4);
  });

  test(`the haze-again step ends when its 100 tiles are answered ${WALK}`, () => {
    let s = run([{ kind: "versions", versions: [V3, V4] }, { kind: "start", step: "after" }]);
    s = traffic(s, 99, 4);
    expect(s.current).toBe("after");
    s = traffic(s, 1, 4, () => true, "z");
    expect(s.current).toBe("end");
  });
});

describe("accuracy", () => {
  test(`counts own tiles per step and serving version ${ACCURACY}`, () => {
    let s = atHaze();
    s = traffic(s, 10, 3, (i) => i < 2);
    s = step(s, completedFor("someone-else", "Forest", 3)); // not ours: shown, not counted
    const rows = accuracy(s);
    expect(rows).toContainEqual({ step: "normal", version: 3, right: WINDOW, total: WINDOW });
    expect(rows).toContainEqual({ step: "haze", version: 3, right: 2, total: 10 });
    expect(s.foreign).toBe(1);
  });

  test(`a tile still in flight when its step ends counts for that step ${ACCURACY}`, () => {
    let s = atHaze();
    s = step(s, ev("DriftDetected", { versionId: "v3", version: 3, features: [] }, "MONITOR"));
    expect(s.current).toBe("retrain");
    s = step(s, { kind: "sent", jobId: "late", tile: "t", cls: "Forest", variant: "haze", step: "haze" });
    s = step(s, completedFor("late", "SeaLake", 3));
    expect(accuracy(s).find((r) => r.step === "retrain")).toBeUndefined();
    expect(s.steps.haze.sent).toBe(1);
  });

  test(`a completion that arrives before its job id is matched later ${ACCURACY}`, () => {
    let s = run([{ kind: "versions", versions: [V3] }, { kind: "start", step: "normal" }]);
    s = step(s, completedFor("early", "Forest", 3));
    expect(s.foreign).toBe(1);
    s = step(s, { kind: "sent", jobId: "early", tile: "t", cls: "Forest", variant: "clean" });
    expect(s.foreign).toBe(0);
    expect(accuracy(s)).toEqual([{ step: "normal", version: 3, right: 1, total: 1 }]);
  });

  test(`repeated events are ignored ${ACCURACY}`, () => {
    let s = run([{ kind: "versions", versions: [V3] }, { kind: "start", step: "normal" }, { kind: "sent", jobId: "a", tile: "t", cls: "Forest", variant: "clean" }]);
    const done = completedFor("a", "Forest", 3);
    s = step(step(s, done), done);
    expect(s.steps.normal.completed).toBe(1);
  });
});

test(`no input approves or promotes on its own ${NEVER}`, () => {
  // The reducer only records an approval a person made; reaching the decision step changes
  // nothing in the registry, and the state has no way to ask it to.
  let s = run([{ kind: "versions", versions: [V3, V4] }, { kind: "start", step: "decide", candidateId: "v4" }]);
  s = step(s, { kind: "gates", versionId: "v4", gates: [{ name: "approval", passed: false, detail: "0 approval(s) recorded" }] });
  expect(s.approval).toBeUndefined();
  expect(s.current).toBe("decide");
  s = step(s, { kind: "approved", versionId: "v4", approver: "A. Person", reason: "gates passed, accuracy checked" });
  expect(s.approval?.approver).toBe("A. Person");
  expect(s.current).toBe("decide"); // promotion is a separate action
});
