// Reads the dated benchmark reports (spec: dashboard "Results come from dated reports").
// Every number the results page shows comes from these files; only the descriptions of
// what each run was are written here.

import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";

export const REPORTS_DIR = join(process.cwd(), "content", "benchmarks");

export interface Latency {
  batch: number;
  iterations: number;
  p50_ms: number;
  p95_ms: number;
  p99_ms: number;
  images_per_s: number;
}

export interface VariantsReport {
  date: string;
  holdout_tiles: number;
  environment: { cpu: string; logical_cores: number; threads: number; onnxruntime: string; torch: string };
  parity: { max_abs_diff: number; disagreements: number };
  variants: { name: string; size_bytes: number; accuracy: number; macro_f1: number; latency: Latency[] }[];
}

export interface LoadReport {
  file: string;
  date: string;
  closed_loop?: boolean;
  concurrency: number;
  duration_s: number;
  report: {
    submitted: number;
    failed_submissions: number;
    completed: number;
    lost: number;
    completed_per_second: number;
    p50_ms: number;
    p95_ms: number;
    p99_ms: number;
    max_ms: number;
  };
}

export interface DriftReport {
  date: string;
  window: number;
  evaluate_every: number;
  psi_enter: number;
  psi_exit: number;
  clean_events: number;
  false_alarms: number;
  cap: number;
  conditions: {
    degradation: string;
    strength: number;
    detected: boolean;
    degraded_events_until_detection: number | null;
    drifted_features: string[];
  }[];
}

export interface PromotionReport {
  date: string;
  report: string;
  method: string;
  shadow: { production_version: number; shadow_version: number; comparisons: number; agreeing: number; mean_shadow_ms: number };
  promote_to_serving_ms: number;
  rollback_to_serving_ms: number;
  rollback_under_load: { submitted: number; failed: number; lost: number };
}

export interface FailureRun {
  file: string;
  date: string;
  mode: "async" | "sync";
  fault: string;
  config: { concurrency: number; duration_s: number; fault_at_s: number; heal_at_s: number };
  events: { injected_at_s: number; healed_at_s: number; inject_error?: string; heal_error?: string };
  report: {
    requests: number;
    rejected: number;
    completed: number;
    lost: number;
    duplicates: number;
    p50_ms: number;
    p99_ms: number;
    max_ms: number;
    baseline_per_second: number;
    recovery_seconds: number | null;
    completed_per_second: number[];
    rejected_per_second: number[];
  };
}

export const FAULTS = ["none", "worker-kill", "nats-restart", "postgres-pause", "objectstore-pause"] as const;

export interface Results {
  variants: VariantsReport;
  load: LoadReport[];
  drift: DriftReport;
  promotion: PromotionReport;
  spike: { autoscaled: LoadReport; oneWorker: LoadReport };
  failureLab: FailureRun[]; // empty until the failure lab has run
}

/** What each load run was; the file name is the key, the numbers come from the file. */
export const LOAD_RUNS: Record<string, string> = {
  "load-closed-loop-pytorch-fp32": "PyTorch fp32, one worker",
  "load-closed-loop-onnx-int8": "ONNX int8, one worker",
  "load-closed-loop-onnx-int8-two-workers-fetch1": "ONNX int8, two workers, one message per fetch",
  "load-closed-loop-onnx-int8-two-workers-fetch8": "ONNX int8, two workers, eight messages per fetch",
  "load-open-loop-overload-pytorch-fp32": "Open loop at 8 clients, past capacity",
  "load-shadow-spinning-threads": "Shadow loaded, default ONNX Runtime threads",
  "load-shadow-single-thread": "Shadow loaded, one thread per model",
  "load-rollback-under-load": "Rollback in the middle of the run",
};

function read<T>(dir: string, file: string): T {
  return JSON.parse(readFileSync(/*turbopackIgnore: true*/ join(dir, file), "utf-8")) as T;
}

/** The newest report whose name ends with `suffix`. */
function latest(files: string[], suffix: string): string {
  const match = files.filter((f) => f.endsWith(`${suffix}.json`)).sort();
  if (!match.length) throw new Error(`no report *${suffix}.json in benchmarks`);
  return match[match.length - 1];
}

/** The run key of a report file: its name without the date and extension. */
export const runKey = (file: string) => file.replace(/^\d{4}-\d{2}-\d{2}-/, "").replace(/\.json$/, "");

/** The newest run of each mode and fault, in FAULTS order, async before sync. */
function failureLab(dir: string, files: string[]): FailureRun[] {
  const out: FailureRun[] = [];
  for (const fault of FAULTS) {
    for (const mode of ["async", "sync"] as const) {
      const match = files.filter((f) => f.endsWith(`-failure-lab-${mode}-${fault}.json`)).sort();
      if (match.length) out.push({ ...read<FailureRun>(dir, match[match.length - 1]), file: match[match.length - 1] });
    }
  }
  return out;
}

export function loadResults(dir = REPORTS_DIR): Results {
  const files = readdirSync(/*turbopackIgnore: true*/ dir);
  const load = Object.keys(LOAD_RUNS).map((key) => ({ ...read<LoadReport>(dir, latest(files, key)), file: latest(files, key) }));
  return {
    variants: read<VariantsReport>(dir, latest(files, "model-variants")),
    load,
    drift: read<DriftReport>(dir, latest(files, "drift-detection")),
    promotion: read<PromotionReport>(dir, latest(files, "promotion-run")),
    spike: {
      autoscaled: { ...read<LoadReport>(dir, latest(files, "k8s-spike-autoscaled")), file: latest(files, "k8s-spike-autoscaled") },
      oneWorker: { ...read<LoadReport>(dir, latest(files, "k8s-spike-one-worker")), file: latest(files, "k8s-spike-one-worker") },
    },
    failureLab: failureLab(dir, files),
  };
}

/** The date of a report, from its date field (a date or a timestamp). */
export const reportDate = (date: string) => date.slice(0, 10);
