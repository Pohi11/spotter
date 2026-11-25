import { expect, test } from "vitest";

import { type Entry, fmtDuration, lengths, schedule } from "./replay";
import { initialState, step } from "./scenario";
import { spec } from "./spec";

const REPLAY = spec("dashboard", "The demo page replays a recording when the platform is unreachable");
const SAVED = spec("dashboard", "A live demo run can be saved as a recording");

const start: Entry = { t: 0, input: { kind: "start", step: "normal" } };

test(`a long wait is shortened and its real length announced ${REPLAY}`, () => {
  const entries: Entry[] = [
    start,
    { t: 500, input: { kind: "versions", versions: [] } },
    { t: 500 + 7 * 60_000 + 4_000, input: { kind: "gates", versionId: "v", gates: [] } },
    { t: 500 + 7 * 60_000 + 5_000, input: { kind: "versions", versions: [] } },
  ];
  const s = schedule(entries);
  expect(s.map((x) => x.input.kind)).toEqual(["start", "versions", "skipped", "gates", "versions"]);
  expect(s[2].input).toEqual({ kind: "skipped", ms: 7 * 60_000 + 4_000 });
  expect(s[3].at).toBe(500 + 3_000);
  expect(s[4].at).toBe(500 + 3_000 + 1_000);
  expect(fmtDuration(7 * 60_000 + 4_000)).toBe("7 min 4 s");
  const { realMs, replayMs } = lengths(entries);
  expect(realMs).toBe(entries[3].t);
  expect(replayMs).toBe(4_500);
});

test(`gaps up to 20 seconds keep their recorded pace ${REPLAY}`, () => {
  const s = schedule([start, { t: 20_000, input: { kind: "versions", versions: [] } }]);
  expect(s.map((x) => x.at)).toEqual([0, 20_000]);
});

test(`replaying a saved run reaches the same state ${SAVED}`, () => {
  const entries: Entry[] = [
    { t: 0, input: { kind: "versions", versions: [{ id: "v3", version: 3, stage: "MODEL_VERSION_STAGE_PRODUCTION", framework: "onnx", metrics: {} }] } },
    { t: 10, input: { kind: "start", step: "normal" } },
    { t: 20, input: { kind: "sent", jobId: "a", tile: "t", cls: "Forest", variant: "clean" } },
  ];
  const live = entries.map((e) => e.input).reduce(step, initialState());
  const replayed = schedule(entries).map((x) => x.input).reduce(step, initialState());
  expect(replayed.steps).toEqual(live.steps);
  expect(replayed.current).toBe(live.current);
});

test(`a shortened wait is attributed to the step it happened in ${REPLAY}`, () => {
  let s = initialState();
  s = step(s, { kind: "start", step: "retrain" });
  s = step(s, { kind: "skipped", ms: 300_000 });
  s = step(s, { kind: "skipped", ms: 60_000 });
  expect(s.skips).toEqual({ retrain: 360_000 });
});
