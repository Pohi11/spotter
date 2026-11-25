// Recordings of live demo runs and their replay (spec: dashboard "A live demo run can be
// saved as a recording" and "The demo page replays a recording when the platform is
// unreachable"). A recording is the list of inputs a live run fed to the reducer, each
// with its time since the run began, so a replay runs the same logic on the same inputs.

import type { Input } from "@/lib/scenario";

export interface Entry {
  /** Milliseconds since the run began. */
  t: number;
  input: Input;
}

export interface Recording {
  /** ISO time the run began. */
  recordedAt: string;
  model: string;
  about: string;
  entries: Entry[];
}

export const MAX_GAP_MS = 20_000;
export const CUT_TO_MS = 3_000;

export interface Scheduled {
  /** When to feed the input, in replay milliseconds at 1× speed. */
  at: number;
  input: Input;
}

/**
 * The replay's timeline: recorded pace, except that a gap longer than MAX_GAP_MS is cut to
 * CUT_TO_MS and announced by a `skipped` input carrying the gap's real length.
 */
export function schedule(entries: Entry[], maxGap = MAX_GAP_MS, cutTo = CUT_TO_MS): Scheduled[] {
  const out: Scheduled[] = [];
  let at = 0;
  let prev = entries[0]?.t ?? 0;
  for (const e of entries) {
    const gap = e.t - prev;
    if (gap > maxGap) {
      out.push({ at: at + 1, input: { kind: "skipped", ms: gap } });
      at += cutTo;
    } else {
      at += Math.max(0, gap);
    }
    out.push({ at, input: e.input });
    prev = e.t;
  }
  return out;
}

/** The recording's real length and its replay length at 1×. */
export function lengths(entries: Entry[]): { realMs: number; replayMs: number } {
  const s = schedule(entries);
  return { realMs: entries.length ? entries[entries.length - 1].t - entries[0].t : 0, replayMs: s.length ? s[s.length - 1].at : 0 };
}

export function fmtDuration(ms: number): string {
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s} s`;
  const m = Math.floor(s / 60);
  return s % 60 ? `${m} min ${s % 60} s` : `${m} min`;
}
