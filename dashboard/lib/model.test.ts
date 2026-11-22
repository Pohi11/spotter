import { expect, test } from "vitest";

import { blockingGates, canPromote, rollbackTarget } from "./model";
import { spec } from "./spec";
import type { Gate, ModelVersion, Stage } from "./types";

const v = (version: number, stage: Stage, updatedAt: string): ModelVersion => ({
  id: `id-${version}`,
  modelId: "m",
  version,
  artifactUri: "",
  artifactSha256: "",
  framework: "onnx",
  metrics: { accuracy: 0.96 },
  stage,
  revision: "1",
  createdAt: updatedAt,
  updatedAt,
});

const gates = (approval: boolean, agreement = true): Gate[] => [
  { name: "accuracy", passed: true, detail: "" },
  { name: "shadow_agreement", passed: agreement, detail: "120 of at least 200 comparisons" },
  { name: "shadow_latency", passed: true, detail: "" },
  { name: "approval", passed: approval, detail: "" },
];

const PROMOTE = spec("dashboard", "Promotion is shown gate by gate and needs a person");

test(`promote is offered only when every gate passes ${PROMOTE}`, () => {
  expect(canPromote(gates(true))).toBe(true);
  expect(canPromote(gates(false))).toBe(false);
  expect(canPromote(gates(true, false))).toBe(false);
  expect(canPromote([])).toBe(false);
});

test(`a failed gate other than approval is reported as blocking ${PROMOTE}`, () => {
  expect(blockingGates(gates(false, false)).map((g) => g.name)).toEqual(["shadow_agreement"]);
  expect(blockingGates(gates(false))).toEqual([]);
});

const ROLLBACK = spec("dashboard", "Rollback names what it will restore and asks first");

test(`rollback restores the version most recently retired from production ${ROLLBACK}`, () => {
  const versions = [
    v(3, "MODEL_VERSION_STAGE_PRODUCTION", "2026-11-22T20:00:00Z"),
    v(2, "MODEL_VERSION_STAGE_RETIRED", "2026-11-22T19:00:00Z"),
    v(1, "MODEL_VERSION_STAGE_RETIRED", "2026-11-22T18:00:00Z"),
  ];
  expect(rollbackTarget(versions)?.version).toBe(2);
});

test(`no rollback without a production version ${ROLLBACK}`, () => {
  expect(rollbackTarget([v(2, "MODEL_VERSION_STAGE_RETIRED", "2026-11-22T19:00:00Z")])).toBeUndefined();
});
