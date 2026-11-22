// Stage names and the rules the model page shows (spec: dashboard). The registry enforces
// every rule; these functions only decide what the page offers.

import type { Gate, ModelVersion, Stage } from "@/lib/types";

const NAMES: Record<Stage, string> = {
  MODEL_VERSION_STAGE_REGISTERED: "registered",
  MODEL_VERSION_STAGE_VALIDATED: "validated",
  MODEL_VERSION_STAGE_SHADOW: "shadow",
  MODEL_VERSION_STAGE_PRODUCTION: "production",
  MODEL_VERSION_STAGE_RETIRED: "retired",
  MODEL_VERSION_STAGE_REJECTED: "rejected",
  MODEL_VERSION_STAGE_ROLLED_BACK: "rolled back",
};

export function stageName(stage: Stage | undefined): string {
  return stage ? (NAMES[stage] ?? stage) : "?";
}

export type StageTone = "pass" | "wait" | "code" | "fail" | "plain";

export function stageTone(stage: Stage): StageTone {
  switch (stage) {
    case "MODEL_VERSION_STAGE_PRODUCTION":
      return "pass";
    case "MODEL_VERSION_STAGE_SHADOW":
      return "wait";
    case "MODEL_VERSION_STAGE_REGISTERED":
    case "MODEL_VERSION_STAGE_VALIDATED":
      return "code";
    case "MODEL_VERSION_STAGE_REJECTED":
    case "MODEL_VERSION_STAGE_ROLLED_BACK":
      return "fail";
    default:
      return "plain";
  }
}

/** Promote is offered only when every gate passes; the registry checks again. */
export function canPromote(gates: Gate[]): boolean {
  return gates.length > 0 && gates.every((g) => g.passed);
}

/** Gates other than approval that fail: what still blocks promotion besides a person. */
export function blockingGates(gates: Gate[]): Gate[] {
  return gates.filter((g) => !g.passed && g.name !== "approval");
}

export function production(versions: ModelVersion[]): ModelVersion | undefined {
  return versions.find((v) => v.stage === "MODEL_VERSION_STAGE_PRODUCTION");
}

/**
 * The version a rollback would restore: the one most recently retired from production
 * (only production can become retired), or none when nothing is in production.
 */
export function rollbackTarget(versions: ModelVersion[]): ModelVersion | undefined {
  if (!production(versions)) return undefined;
  return versions
    .filter((v) => v.stage === "MODEL_VERSION_STAGE_RETIRED")
    .sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt))[0];
}
