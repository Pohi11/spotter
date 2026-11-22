// One-line summaries of feed events for the live page (spec: dashboard "The live page shows
// events as they arrive").

import { stageName } from "@/lib/model";
import type { FeedEvent, Stage } from "@/lib/types";

export type Tone = "bus" | "code" | "model" | "pass" | "wait" | "human" | "fail" | "plain";

export interface Summary {
  title: string;
  detail: string;
  tone: Tone;
}

const short = (id: unknown) => (typeof id === "string" ? id.slice(-8) : "?");
const pct = (n: unknown) => (typeof n === "number" ? `${(n * 100).toFixed(1)}%` : "?");

type P = Record<string, unknown>;

export function summarize(ev: FeedEvent): Summary {
  const p = (ev.payload ?? {}) as P;
  const pred = (p.prediction ?? {}) as P;
  switch (ev.type) {
    case "spotter.v1.InferenceRequested":
      return { title: "Job requested", detail: `job …${short(p.jobId)} for ${p.modelName}`, tone: "code" };
    case "spotter.v1.InferenceCompleted":
      return {
        title: "Job completed",
        detail: `job …${short(p.jobId)}: ${pred.label} at ${pct(pred.confidence)} by version ${pred.modelVersion}`,
        tone: "model",
      };
    case "spotter.v1.InferenceFailed":
      return { title: "Job failed", detail: `job …${short(p.jobId)}: ${p.reason || "no reason given"}`, tone: "fail" };
    case "spotter.v1.ShadowCompared":
      return {
        title: "Shadow compared",
        detail: `job …${short(p.jobId)}: shadow said ${p.shadowLabel}, production ${p.productionLabel}`,
        tone: p.agree ? "wait" : "fail",
      };
    case "spotter.v1.ModelVersionRegistered":
      return { title: "Version registered", detail: `${p.modelName} version ${p.version}`, tone: "code" };
    case "spotter.v1.ModelVersionStageChanged": {
      const to = p.toStage as Stage | undefined;
      return {
        title: "Stage changed",
        detail: `version ${p.version}: ${stageName(p.fromStage as Stage)} to ${stageName(to)}${p.reason ? `, “${p.reason}”` : ""}`,
        tone: to === "MODEL_VERSION_STAGE_PRODUCTION" ? "pass" : to === "MODEL_VERSION_STAGE_ROLLED_BACK" ? "fail" : "code",
      };
    }
    case "spotter.v1.DriftDetected":
      return { title: "Drift detected", detail: `version ${p.version}: ${features(p)}`, tone: "fail" };
    case "spotter.v1.DriftResolved":
      return { title: "Drift resolved", detail: `version ${p.version}`, tone: "pass" };
    case "spotter.v1.DriftEvaluated":
      return {
        title: "Drift evaluated",
        detail: `version ${p.version}: ${p.drifting ? `drifting on ${features(p, 0.2)}` : "no drift"}`,
        tone: p.drifting ? "fail" : "bus",
      };
    default:
      return { title: ev.type || "Unknown event", detail: ev.subject, tone: "plain" };
  }
}

/** The features listed in a drift payload, optionally only those at or above a PSI. */
function features(p: P, atLeast = 0): string {
  const list = ((p.features ?? []) as { feature: string; psi: number }[]).filter((f) => f.psi >= atLeast);
  return list.map((f) => `${f.feature} PSI ${f.psi.toFixed(2)}`).join(", ") || "no features";
}

/** Adds an event to a newest-first list, ignoring repeats (a reconnect replays recent events). */
export function addEvent(list: FeedEvent[], ev: FeedEvent, limit = 200): FeedEvent[] {
  if (list.some((e) => e.stream === ev.stream && e.sequence === ev.sequence)) return list;
  const next = [ev, ...list];
  next.sort((a, b) => Date.parse(b.time) - Date.parse(a.time));
  return next.slice(0, limit);
}

export function countByStream(list: FeedEvent[]): Record<string, number> {
  const counts: Record<string, number> = { INFERENCE: 0, REGISTRY: 0, MONITOR: 0 };
  for (const e of list) counts[e.stream] = (counts[e.stream] ?? 0) + 1;
  return counts;
}
