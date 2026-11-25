// The guided demo's state (design D4 of change phase-14-guided-demo): a pure reducer from
// inputs to what the page shows. A live run produces inputs from the tiles it sends, the
// event feed, and the registry; a replay produces the same inputs from a recording. Steps
// complete only on the platform's evidence (spec: dashboard "Demo steps complete on the
// platform's evidence").

import { compact, explain } from "@/lib/explain";
import type { DriftEvaluated, FeatureDrift, FeedEvent, Gate, Stage } from "@/lib/types";

export type StepId = "normal" | "haze" | "retrain" | "shadow" | "decide" | "after";
export const STEP_ORDER: StepId[] = ["normal", "haze", "retrain", "shadow", "decide", "after"];

/** What each sending step sends, and at most how many (bundles cycle past 300 tiles). */
export const SENDS: Partial<Record<StepId, { variant: string; max: number }>> = {
  normal: { variant: "clean", max: 600 },
  haze: { variant: "haze", max: 600 },
  shadow: { variant: "clean", max: 900 },
  after: { variant: "haze", max: 100 },
};

export const WINDOW = 200; // the monitor's window, in completions

export interface VersionInfo {
  id: string;
  version: number;
  stage: Stage;
  framework: string;
  metrics: Record<string, number>;
}

export type Input =
  | { kind: "event"; ev: FeedEvent }
  /** A tile the gateway accepted, for the step that sent it (the step may have ended since). */
  | { kind: "sent"; jobId: string; tile: string; cls: string; variant: string; step?: StepId }
  | { kind: "versions"; versions: VersionInfo[] }
  | { kind: "gates"; versionId: string; gates: Gate[] }
  /** Start the current sending step, or skip ahead to a later step (optionally naming the candidate). */
  | { kind: "start"; step: StepId; candidateId?: string }
  | { kind: "approved"; versionId: string; approver: string; reason: string }
  /** Replay only: a long gap in the recording was shortened. */
  | { kind: "skipped"; ms: number };

export type StepStatus = "pending" | "ready" | "running" | "done" | "skipped";

export interface StepState {
  status: StepStatus;
  sent: number;
  completed: number;
  /** What the evidence said when the step finished. */
  result?: string;
  /** What the step is waiting on, when it isn't obvious. */
  note?: string;
}

export interface JobInfo {
  tile: string;
  cls: string;
  variant: string;
  step: StepId;
  label?: string;
  version?: number;
}

export interface State {
  current: StepId | "end";
  steps: Record<StepId, StepState>;
  jobs: Record<string, JobInfo>;
  /** Completions that arrived before the page learned their job id. */
  orphans: Record<string, { label: string; version: number }>;
  seen: Set<string>;
  /** Newest first, at most EVENT_LIMIT. */
  events: FeedEvent[];
  published: Record<string, number>;
  foreign: number;
  versions: VersionInfo[];
  candidate?: { id: string; version: number };
  gates?: Gate[];
  /** The latest evaluation per version id, and the latest of all. */
  evaluations: Record<string, DriftEvaluated>;
  lastEvaluation?: DriftEvaluated;
  approval?: { versionId: string; approver: string; reason: string };
  /** The page's own most recent jobs, newest first. */
  recent: string[];
  /** The production version when the haze step started: the one whose drift we wait for. */
  watched?: { id: string; version: number };
  skippedMs?: number;
}

export const EVENT_LIMIT = 150;
export const RECENT = 16;

const freshStep = (status: StepStatus = "pending"): StepState => ({ status, sent: 0, completed: 0 });

export function initialState(): State {
  const steps = Object.fromEntries(STEP_ORDER.map((s) => [s, freshStep()])) as Record<StepId, StepState>;
  steps.normal.status = "ready";
  return { current: "normal", steps, jobs: {}, orphans: {}, seen: new Set(), events: [], published: {}, foreign: 0, versions: [], evaluations: {}, recent: [] };
}

export const production = (s: State) => s.versions.find((v) => v.stage === "MODEL_VERSION_STAGE_PRODUCTION");

/** Shadow versions newer than production: candidates trained before the demo reached step 3. */
export function shadowCandidates(s: State): VersionInfo[] {
  const prod = production(s);
  return s.versions.filter((v) => v.stage === "MODEL_VERSION_STAGE_SHADOW" && (!prod || v.version > prod.version)).sort((a, b) => b.version - a.version);
}

/** Whether the sender should be submitting tiles for the current step. */
export function sending(s: State): { variant: string; remaining: number } | null {
  if (s.current === "end") return null;
  const plan = SENDS[s.current];
  const st = s.steps[s.current];
  if (!plan || st.status !== "running" || st.sent >= plan.max) return null;
  return { variant: plan.variant, remaining: plan.max - st.sent };
}

type P = Record<string, unknown>;

function setStep(s: State, id: StepId, patch: Partial<StepState>): State {
  return { ...s, steps: { ...s.steps, [id]: { ...s.steps[id], ...patch } } };
}

/** Marks the current step done and makes the next one current. */
function finish(s: State, id: StepId, result: string): State {
  let next = setStep(s, id, { status: "done", result, note: undefined });
  const i = STEP_ORDER.indexOf(id);
  const following = STEP_ORDER[i + 1];
  if (!following) return { ...next, current: "end" };
  // Waiting steps run as soon as they're current; sending steps wait for a click.
  next = setStep(next, following, { status: SENDS[following] ? "ready" : "running" });
  return { ...next, current: following };
}

function applyStage(versions: VersionInfo[], p: P): VersionInfo[] {
  const id = p.versionId as string;
  const to = p.toStage as Stage;
  if (!versions.some((v) => v.id === id)) {
    return [...versions, { id, version: Number(p.version), stage: to, framework: "", metrics: {} }];
  }
  return versions.map((v) => (v.id === id ? { ...v, stage: to } : v));
}

function complete(s: State, jobId: string, label: string, version: number): State {
  const job = s.jobs[jobId];
  if (!job) {
    return { ...s, foreign: s.foreign + 1, orphans: { ...s.orphans, [jobId]: { label, version } } };
  }
  if (job.label !== undefined) return s; // already counted
  const st = s.steps[job.step];
  return { ...setStep(s, job.step, { completed: st.completed + 1 }), jobs: { ...s.jobs, [jobId]: { ...job, label, version } } };
}

function maxPsi(fs: FeatureDrift[]): FeatureDrift | undefined {
  return fs.reduce<FeatureDrift | undefined>((a, f) => (!a || f.psi > a.psi ? f : a), undefined);
}

function onEvent(s0: State, raw: FeedEvent): State {
  const key = `${raw.stream}:${raw.sequence}`;
  if (s0.seen.has(key)) return s0; // a reconnect replays the recent window
  const ev = compact(raw);
  const seen = new Set(s0.seen).add(key);
  const pub = explain(ev).publisher;
  let s: State = { ...s0, seen, events: [ev, ...s0.events].slice(0, EVENT_LIMIT), published: { ...s0.published, [pub]: (s0.published[pub] ?? 0) + 1 } };
  const p = (ev.payload ?? {}) as P;
  const prod = production(s);

  switch (ev.type) {
    case "spotter.v1.InferenceCompleted": {
      const pred = (p.prediction ?? {}) as P;
      return complete(s, p.jobId as string, pred.label as string, Number(pred.modelVersion));
    }
    case "spotter.v1.DriftEvaluated": {
      const e = p as unknown as DriftEvaluated;
      s = { ...s, evaluations: { ...s.evaluations, [e.versionId]: e }, lastEvaluation: e };
      const st = s.steps.normal;
      if (s.current === "normal" && st.status === "running" && prod && e.versionId === prod.id) {
        if (e.drifting) return setStep(s, "normal", { note: `Version ${e.version} is still drifting from earlier traffic. Clean tiles keep going until its PSI falls below 0.1 and the monitor resolves it.` });
        if (st.completed >= WINDOW) {
          const top = maxPsi(e.features);
          return finish(s, "normal", `Version ${e.version}'s last ${e.windowSize} inputs match its training reference: highest PSI ${top ? `${top.psi.toFixed(3)} (${top.feature})` : "?"}, under the 0.2 threshold.`);
        }
      }
      return s;
    }
    case "spotter.v1.DriftDetected": {
      const watched = s.watched ?? (prod && { id: prod.id, version: prod.version });
      if (s.current === "haze" && s.steps.haze.status === "running" && watched && p.versionId === watched.id) {
        const fs = ((p.features ?? []) as FeatureDrift[]).map((f) => `${f.feature} ${f.psi.toFixed(2)}`).join(", ");
        return finish(s, "haze", `Drift detected on version ${p.version} after ${s.steps.haze.completed} hazed tiles completed: ${fs}.`);
      }
      return s;
    }
    case "spotter.v1.ModelVersionRegistered": {
      if (s.versions.some((v) => v.id === p.versionId)) return s;
      return { ...s, versions: [...s.versions, { id: p.versionId as string, version: Number(p.version), stage: "MODEL_VERSION_STAGE_REGISTERED", framework: (p.framework as string) ?? "", metrics: {} }] };
    }
    case "spotter.v1.ModelVersionStageChanged": {
      s = { ...s, versions: applyStage(s.versions, p) };
      const to = p.toStage as Stage;
      const version = Number(p.version);
      if (to === "MODEL_VERSION_STAGE_SHADOW" && s.current === "retrain" && (!prod || version > prod.version)) {
        s = { ...s, candidate: { id: p.versionId as string, version }, gates: undefined };
        return finish(s, "retrain", `Version ${version} reached shadow without anyone starting it: registered, validated against the accuracy floor, then moved to shadow.`);
      }
      if (to === "MODEL_VERSION_STAGE_PRODUCTION" && s.current === "decide" && s.candidate?.id === p.versionId) {
        return finish(s, "decide", `Version ${version} is in production. ${p.reason ? `Recorded reason: “${p.reason}”.` : ""}`.trim());
      }
      return s;
    }
    default:
      return s;
  }
}

export function reduce(s: State, input: Input): State {
  switch (input.kind) {
    case "event":
      return onEvent(s, input.ev);
    case "sent": {
      if (s.jobs[input.jobId]) return s;
      const step = input.step ?? (s.current === "end" ? undefined : s.current);
      if (!step) return s;
      let next: State = {
        ...setStep(s, step, { sent: s.steps[step].sent + 1 }),
        jobs: { ...s.jobs, [input.jobId]: { tile: input.tile, cls: input.cls, variant: input.variant, step } },
        recent: [input.jobId, ...s.recent].slice(0, RECENT),
      };
      const early = next.orphans[input.jobId];
      if (early) {
        const orphans = { ...next.orphans };
        delete orphans[input.jobId];
        next = complete({ ...next, orphans, foreign: next.foreign - 1 }, input.jobId, early.label, early.version);
      }
      return next;
    }
    case "versions":
      return { ...s, versions: input.versions };
    case "gates": {
      if (s.candidate?.id !== input.versionId) return s;
      const next = { ...s, gates: input.gates };
      const agreement = input.gates.find((g) => g.name === "shadow_agreement");
      if (s.current === "shadow" && s.steps.shadow.status === "running" && agreement?.passed) {
        return finish(next, "shadow", `Agreement gate passed: ${agreement.detail}.`);
      }
      return next;
    }
    case "start":
      return start(s, input.step, input.candidateId);
    case "approved":
      return { ...s, approval: { versionId: input.versionId, approver: input.approver, reason: input.reason } };
    case "skipped":
      return { ...s, skippedMs: input.ms };
  }
}

function start(s: State, step: StepId, candidateId?: string): State {
  if (s.current === "end") return s;
  const from = STEP_ORDER.indexOf(s.current);
  const to = STEP_ORDER.indexOf(step);
  if (to < from) return s;
  let next = s;
  for (let i = from; i < to; i++) next = setStep(next, STEP_ORDER[i], { status: "skipped", note: undefined });
  next = setStep(next, step, { status: "running" });
  next = { ...next, current: step };
  if (step === "haze") {
    const prod = production(next);
    next = { ...next, watched: prod && { id: prod.id, version: prod.version } };
  }
  if (candidateId) {
    const v = next.versions.find((x) => x.id === candidateId);
    if (v) next = { ...next, candidate: { id: v.id, version: v.version }, gates: undefined };
  }
  return next;
}

/** Ends the haze-again step once every tile it sent has completed. Called after each input. */
export function settle(s: State): State {
  if (s.current !== "after") return s;
  const st = s.steps.after;
  const plan = SENDS.after!;
  if (st.status === "running" && st.sent >= plan.max && st.completed >= st.sent) {
    return finish(s, "after", `All ${st.completed} hazed tiles were answered by the new version.`);
  }
  return s;
}

/** reduce, then settle: what the page dispatches. */
export function step(s: State, input: Input): State {
  return settle(reduce(s, input));
}

export interface Accuracy {
  step: StepId;
  version: number;
  right: number;
  total: number;
}

/** Accuracy of the page's own completed tiles, per step and serving version. */
export function accuracy(s: State): Accuracy[] {
  const rows = new Map<string, Accuracy>();
  for (const job of Object.values(s.jobs)) {
    if (job.label === undefined || job.version === undefined) continue;
    const key = `${job.step}|${job.version}`;
    const row = rows.get(key) ?? { step: job.step, version: job.version, right: 0, total: 0 };
    row.total++;
    if (job.label === job.cls) row.right++;
    rows.set(key, row);
  }
  return [...rows.values()].sort((a, b) => STEP_ORDER.indexOf(a.step) - STEP_ORDER.indexOf(b.step) || a.version - b.version);
}

/** The evaluation the drift panel shows: production's latest, else the latest of any version. */
export function watchedEvaluation(s: State): DriftEvaluated | undefined {
  const prod = production(s);
  return (prod && s.evaluations[prod.id]) ?? (s.watched && s.evaluations[s.watched.id]) ?? s.lastEvaluation;
}
