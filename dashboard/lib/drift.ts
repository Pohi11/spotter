// Turns DriftEvaluated events into per-version PSI series (spec: dashboard "The drift page
// charts PSI over time"). Thresholds match the monitor's (decision record 0012).

import type { DriftEvaluated, FeedEvent } from "@/lib/types";

export const DETECT = 0.2;
export const RESOLVE = 0.1;
export const FEATURES = ["mean_r", "mean_g", "mean_b", "contrast", "confidence"] as const;

export const FEATURE_LABELS: Record<string, string> = {
  mean_r: "Red brightness",
  mean_g: "Green brightness",
  mean_b: "Blue brightness",
  contrast: "Contrast",
  confidence: "Model confidence",
};

export interface Point {
  t: number;
  [feature: string]: number;
}

export interface VersionDrift {
  versionId: string;
  version: number;
  drifting: boolean;
  points: Point[];
  latest: DriftEvaluated;
}

export function evaluations(events: FeedEvent[]): DriftEvaluated[] {
  return events
    .filter((e) => e.type === "spotter.v1.DriftEvaluated" && e.payload)
    .map((e) => e.payload as unknown as DriftEvaluated);
}

/** One series per version, oldest point first, with the latest evaluation's state. */
export function byVersion(evals: DriftEvaluated[]): VersionDrift[] {
  const map = new Map<string, DriftEvaluated[]>();
  for (const e of evals) map.set(e.versionId, [...(map.get(e.versionId) ?? []), e]);
  const out: VersionDrift[] = [];
  for (const [versionId, list] of map) {
    list.sort((a, b) => Date.parse(a.evaluatedAt) - Date.parse(b.evaluatedAt));
    const latest = list[list.length - 1];
    out.push({
      versionId,
      version: latest.version,
      drifting: latest.drifting,
      latest,
      points: list.map((e) => {
        const p: Point = { t: Date.parse(e.evaluatedAt) };
        for (const f of e.features) p[f.feature] = f.psi;
        return p;
      }),
    });
  }
  return out.sort((a, b) => b.version - a.version);
}
