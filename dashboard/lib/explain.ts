// Why each event happened and what reacts to it, for the demo page (spec: dashboard "The demo
// page explains every event it sees"). Each sentence describes what the code does; the
// references are to the services that publish and consume the subject.

import { className } from "@/lib/classes";
import { stageName } from "@/lib/model";
import { ENTER, featureName, worst } from "@/lib/psi";
import type { FeatureDrift, FeedEvent, Stage } from "@/lib/types";

export type Component = "gateway" | "worker" | "monitor" | "registry" | "other";

export const COMPONENTS: { id: Exclude<Component, "other">; name: string; does: string }[] = [
  { id: "gateway", name: "Gateway", does: "takes the tile, stores it, queues the job" },
  { id: "worker", name: "Inference worker", does: "classifies with production, and with shadow" },
  { id: "monitor", name: "Drift monitor", does: "compares recent inputs with training" },
  { id: "registry", name: "Registry", does: "versions, stages, gates; outbox to the bus" },
];

export interface Explained {
  publisher: Component;
  title: string;
  /** Why it was published, then who consumes it. */
  why: string;
  /** Registry and monitor events carry the story; per-job events are traffic. */
  story: boolean;
}

type P = Record<string, unknown>;
const pctOf = (n: unknown) => (typeof n === "number" ? `${(n * 100).toFixed(1)}%` : "?");
const feats = (p: P) => (p.features ?? []) as FeatureDrift[];
const list = (fs: FeatureDrift[]) => fs.map((f) => `${featureName(f.feature)} (PSI ${f.psi.toFixed(2)})`).join(", ");

const STAGE_WHY: Partial<Record<Stage, string>> = {
  MODEL_VERSION_STAGE_VALIDATED: "Its holdout accuracy met the configured floor, checked in code before the move.",
  MODEL_VERSION_STAGE_SHADOW: "The worker follows stage changes: it loads this version as its shadow and runs it on every job after answering with production.",
  MODEL_VERSION_STAGE_PRODUCTION: "Promotion retired the old version and promoted this one in one transaction. The worker swaps models when this event reaches it.",
  MODEL_VERSION_STAGE_RETIRED: "Retired by a promotion in the same transaction; a rollback could restore it.",
  MODEL_VERSION_STAGE_ROLLED_BACK: "Marked rolled back; the previously retired version is back in production.",
  MODEL_VERSION_STAGE_REJECTED: "Rejected; it can't move to any other stage.",
};

export function explain(ev: FeedEvent): Explained {
  const p = (ev.payload ?? {}) as P;
  const pred = (p.prediction ?? {}) as P;
  switch (ev.type) {
    case "spotter.v1.InferenceRequested":
      return {
        publisher: "gateway",
        title: "Job queued",
        why: "The gateway wrote the tile to the object store, set the job to pending in the job bucket, and published this. A worker's durable consumer pulls it next; if the worker dies first, JetStream delivers it again.",
        story: false,
      };
    case "spotter.v1.InferenceCompleted":
      return {
        publisher: "worker",
        title: `Version ${pred.modelVersion} said ${className(pred.label as string)} at ${pctOf(pred.confidence)}`,
        why: `The worker recorded the result only if the job was still pending (compare-and-set), then published this. The monitor adds this tile's brightness, contrast, and the model's confidence to version ${pred.modelVersion}'s window.`,
        story: false,
      };
    case "spotter.v1.ShadowCompared":
      return {
        publisher: "worker",
        title: p.agree ? "Shadow agreed" : `Shadow disagreed: ${className(p.shadowLabel as string)} vs ${className(p.productionLabel as string)}`,
        why: "After acknowledging the job with production's answer, the worker ran the shadow version on the same tile. The registry stores one comparison per job for the agreement gate; the shadow's answer is never returned.",
        story: false,
      };
    case "spotter.v1.InferenceFailed":
      return {
        publisher: "worker",
        title: "Job failed",
        why: `${p.reason || "No reason given"}. The worker records a failure only for undecodable input or a job on its last delivery, and publishes it as a dead letter.`,
        story: true,
      };
    case "spotter.v1.ModelVersionRegistered":
      return {
        publisher: "registry",
        title: `Version ${p.version} registered`,
        why: "A trainer registered a new artifact over gRPC. The registry wrote this event to its outbox in the same transaction as the version, and the relay published it.",
        story: true,
      };
    case "spotter.v1.ModelVersionStageChanged": {
      const to = p.toStage as Stage;
      return {
        publisher: "registry",
        title: `Version ${p.version}: ${stageName(p.fromStage as Stage)} → ${stageName(to)}`,
        why: `${STAGE_WHY[to] ?? "A stage change."}${p.reason ? ` Reason recorded: “${p.reason}”.` : ""}`,
        story: true,
      };
    }
    case "spotter.v1.DriftEvaluated": {
      const top = worst(feats(p));
      return {
        publisher: "monitor",
        title: `Version ${p.version} evaluated: ${p.drifting ? "drifting" : "no drift"}`,
        why: `The monitor compared version ${p.version}'s last ${p.windowSize} completions with its training reference, as it does every 50.${top ? ` Highest PSI: ${featureName(top.feature)} ${top.psi.toFixed(3)} (drift at ${ENTER}).` : ""}`,
        story: true,
      };
    }
    case "spotter.v1.DriftDetected":
      return {
        publisher: "monitor",
        title: `Drift detected on version ${p.version}`,
        why: `PSI reached ${ENTER} on ${list(feats(p)) || "a feature"}. Published once, when the state changes. The retrainer consumes ${ev.subject} and starts fine-tuning a new version.`,
        story: true,
      };
    case "spotter.v1.DriftResolved":
      return {
        publisher: "monitor",
        title: `Drift resolved on version ${p.version}`,
        why: "Every feature's PSI fell below 0.1, so the version is back to normal. Published once, when the state changes.",
        story: true,
      };
    default:
      return { publisher: "other", title: ev.type || "Unknown event", why: ev.subject, story: false };
  }
}

/** The event without fields the demo never reads (per-class scores), to keep recordings small. */
export function compact(ev: FeedEvent): FeedEvent {
  if (ev.type !== "spotter.v1.InferenceCompleted" || !ev.payload) return ev;
  const payload: P = { ...ev.payload };
  const prediction: P = { ...((payload.prediction ?? {}) as P) };
  delete prediction.scores;
  delete payload.inputStats;
  return { ...ev, payload: { ...payload, prediction } };
}
