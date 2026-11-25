// The arithmetic behind one PSI, from the bins the monitor publishes with it (spec: dashboard
// "The demo page shows the arithmetic behind each drift evaluation"). The monitor floors
// every share at 1e-4 before summing, and publishes the floored shares, so the sum here is
// the monitor's own.

import type { FeatureDrift } from "@/lib/types";

export const ENTER = 0.2; // a feature at or above this PSI is drifting
export const EXIT = 0.1; // every feature below this to stop drifting

export const FEATURE_NAMES: Record<string, string> = {
  mean_r: "Mean red",
  mean_g: "Mean green",
  mean_b: "Mean blue",
  contrast: "Contrast",
  confidence: "Model confidence",
};

export const featureName = (f: string) => FEATURE_NAMES[f] ?? f;

export interface Bin {
  /** Lower and upper edge; null for the open-ended outer bins. */
  lo: number | null;
  hi: number | null;
  reference: number;
  window: number;
  /** (window − reference) · ln(window / reference): never negative. */
  contribution: number;
}

export interface Breakdown {
  bins: Bin[];
  sum: number;
}

/** The per-bin contributions to a feature's PSI, or null when the event has no bins. */
export function breakdown(f: FeatureDrift): Breakdown | null {
  const edges = f.binEdges ?? [];
  const r = f.referenceShares ?? [];
  const w = f.windowShares ?? [];
  if (r.length === 0 || r.length !== w.length || edges.length !== r.length - 1) return null;
  const bins = r.map((ref, i) => {
    const win = w[i];
    return { lo: i === 0 ? null : edges[i - 1], hi: i === edges.length ? null : edges[i], reference: ref, window: win, contribution: (win - ref) * Math.log(win / ref) };
  });
  return { bins, sum: bins.reduce((s, b) => s + b.contribution, 0) };
}

/** The feature with the highest PSI. */
export function worst(features: FeatureDrift[]): FeatureDrift | undefined {
  return features.reduce<FeatureDrift | undefined>((best, f) => (!best || f.psi > best.psi ? f : best), undefined);
}

export function binLabel(b: Bin, digits = 3): string {
  const n = (x: number) => x.toFixed(digits);
  if (b.lo === null && b.hi !== null) return `below ${n(b.hi)}`;
  if (b.hi === null && b.lo !== null) return `${n(b.lo)} and up`;
  return `${n(b.lo ?? 0)} to ${n(b.hi ?? 0)}`;
}
