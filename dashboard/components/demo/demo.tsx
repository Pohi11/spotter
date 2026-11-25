"use client";

import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from "react";

import { link } from "@/components/ui";
import { type Entry, type Recording, fmtDuration, lengths, schedule } from "@/lib/replay";
import { type Input, type State, type StepId, type VersionInfo, initialState, sending, step } from "@/lib/scenario";
import type { FeedEvent, Gate, ModelVersion } from "@/lib/types";

import { AccuracyPanel, DriftMath, EventLog, PathStrip, RecentTiles } from "./panels";
import { Steps } from "./steps";

// The guided demo (spec: dashboard; design D1, D4, D8 of change phase-14-guided-demo). One
// reducer; a live run feeds it from the platform and logs every input, a replay feeds it
// from a recording and sends nothing.

export const RECORDING_URL = "/recordings/demo.json";
const MAX_IN_FLIGHT = 8;

interface Tile {
  id: string;
  cls: string;
  jpeg: string;
}

type Availability = "checking" | "live" | "no-model" | "unreachable";

export function Demo({ model }: { model: string }) {
  const [availability, setAvailability] = useState<Availability>("checking");
  const [mode, setMode] = useState<"live" | "replay" | null>(null);
  const [recording, setRecording] = useState<Recording | null>(null);

  useEffect(() => {
    let gone = false;
    fetch(`/api/v1/models/${model}`)
      .then((r) => (r.ok ? "live" : r.status === 404 ? "no-model" : "unreachable"))
      .catch(() => "unreachable" as const)
      .then((a) => {
        if (gone) return;
        setAvailability(a);
        const forced = new URLSearchParams(window.location.search).get("mode");
        if (forced === "replay" || a !== "live") setMode((m) => m ?? (forced === "replay" ? "replay" : null));
        else setMode((m) => m ?? "live");
      });
    fetch(RECORDING_URL)
      .then((r) => (r.ok ? r.json() : null))
      .then((rec) => !gone && setRecording(rec))
      .catch(() => undefined);
    return () => {
      gone = true;
    };
  }, [model]);

  const recorded = recording ? new Date(recording.recordedAt).toLocaleDateString("en-US", { dateStyle: "long" }) : null;

  return (
    <div className="mt-10">
      <div className="flex flex-wrap items-center gap-3" aria-live="polite">
        {availability === "checking" && <p className="text-muted">Checking whether the platform is reachable…</p>}
        {availability === "live" && mode === "live" && (
          <>
            <p className="rounded-full bg-pass-soft px-4 py-1.5 text-sm font-medium text-pass">Live: connected to the running platform</p>
            {recording && (
              <button type="button" onClick={() => setMode("replay")} className={`text-sm ${link}`}>
                Watch the recorded run instead
              </button>
            )}
          </>
        )}
        {mode === "replay" && recording && (
          <>
            <p className="rounded-full bg-wait-soft px-4 py-1.5 text-sm font-medium text-wait" data-testid="replay-banner">
              Recording from {recorded}: nothing is sent to any platform
            </p>
            {availability === "live" && (
              <button type="button" onClick={() => setMode("live")} className={`text-sm ${link}`}>
                Run it live instead
              </button>
            )}
          </>
        )}
      </div>

      {mode === null && availability !== "checking" && (
        <div role="status" className="max-w-2xl rounded-2xl bg-surface p-5 ring-1 ring-rule">
          <p className="font-medium">
            {availability === "no-model"
              ? "The platform is running, but no model is registered yet, so there is nothing to demo live."
              : "The platform can't be reached from this site, so the demo can't run live here."}
          </p>
          <p className="mt-1 text-sm text-muted">
            {availability === "no-model" ? "Train version 1 with the demo's bootstrap step (see the README), then reload. " : "To run it live, start the platform (see the README's “Run it” section) and open this page from its dashboard. "}
            Meanwhile, the recorded run shows the same scenario: every event, tile, and gate report a live run of this page received, at their recorded pace.
          </p>
          {recording ? (
            <button type="button" onClick={() => setMode("replay")} className="mt-4 rounded-full bg-ink px-5 py-2.5 font-medium text-white hover:brightness-125">
              Watch the recorded run from {recorded}
            </button>
          ) : (
            <p className="mt-4 text-sm text-muted">No recording is available either.</p>
          )}
        </div>
      )}

      {mode === "live" && <Scenario key="live" model={model} live />}
      {mode === "replay" && recording && <Scenario key="replay" model={model} live={false} recording={recording} />}
    </div>
  );
}

function versionInfo(v: ModelVersion): VersionInfo {
  return { id: v.id, version: v.version, stage: v.stage, framework: v.framework, metrics: v.metrics };
}

function Scenario({ model, live, recording }: { model: string; live: boolean; recording?: Recording }) {
  const [state, dispatch] = useReducer(step, undefined, initialState);
  const stateRef = useRef<State>(state);
  useEffect(() => {
    stateRef.current = state;
  });
  const log = useRef<Entry[]>([]);
  const t0 = useRef<number>(0);
  const startedAt = useRef<string>("");
  const [rate, setRate] = useState(10);
  const [error, setError] = useState<string | null>(null);
  const bundles = useRef<Record<string, { tiles: Tile[]; byId: Map<string, number> }>>({});
  const [, setLoaded] = useState(0);

  const feed = useCallback(
    (input: Input) => {
      if (live) log.current.push({ t: Date.now() - t0.current, input });
      if (input.kind === "start") setError(null);
      dispatch(input);
    },
    [live],
  );

  const loadBundle = useCallback(async (variant: string) => {
    if (bundles.current[variant]) return bundles.current[variant];
    const b = (await (await fetch(`/scenario/${variant}.json`)).json()) as { tiles: Tile[] };
    bundles.current[variant] = { tiles: b.tiles, byId: new Map(b.tiles.map((t, i) => [t.id, i])) };
    setLoaded((n) => n + 1);
    return bundles.current[variant];
  }, []);

  const image = useCallback((variant: string, tile: string) => {
    const b = bundles.current[variant];
    const i = b?.byId.get(tile);
    return b && i !== undefined ? `data:image/jpeg;base64,${b.tiles[i].jpeg}` : undefined;
  }, []);

  // Live: versions, the event feed, and the recording log.
  useEffect(() => {
    if (!live) return;
    t0.current = Date.now();
    startedAt.current = new Date().toISOString();
    log.current = [];
    let gone = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const loadVersions = async () => {
      try {
        const r = await fetch(`/api/v1/models/${model}/versions`);
        if (!r.ok) return;
        const { versions } = (await r.json()) as { versions: ModelVersion[] };
        if (!gone) feed({ kind: "versions", versions: versions.map(versionInfo) });
      } catch {
        // the feed's reconnect and the next registry event try again
      }
    };
    void loadVersions();
    const source = new EventSource(`/api/v1/events?since=2s`);
    source.onmessage = (m) => {
      try {
        const ev = JSON.parse(m.data) as FeedEvent;
        feed({ kind: "event", ev });
        if (ev.stream === "REGISTRY") {
          clearTimeout(timer);
          timer = setTimeout(loadVersions, 400);
        }
      } catch {
        // the feed only sends JSON
      }
    };
    (window as unknown as { __spotterDemoRecording?: () => Recording }).__spotterDemoRecording = () => record(model, startedAt.current, log.current);
    return () => {
      gone = true;
      clearTimeout(timer);
      source.close();
    };
  }, [live, model, feed]);

  // Live: poll the candidate's gates while they decide the step.
  const candidateId = state.candidate?.id;
  const polling = live && candidateId && (state.current === "shadow" || state.current === "decide");
  useEffect(() => {
    if (!polling || !candidateId) return;
    let last = "";
    const poll = async () => {
      try {
        const r = await fetch(`/api/v1/versions/${candidateId}/gates`);
        if (!r.ok) return;
        const { gates } = (await r.json()) as { gates: Gate[] };
        const key = JSON.stringify(gates);
        if (key !== last) {
          last = key;
          feed({ kind: "gates", versionId: candidateId, gates });
        }
      } catch {
        // try again on the next tick
      }
    };
    void poll();
    const t = setInterval(poll, 2000);
    return () => clearInterval(t);
  }, [polling, candidateId, feed]);

  // Live: the sender, at the chosen pace, while the current step wants tiles.
  const plan = sending(state);
  const sendKey = live && plan ? `${state.current}:${plan.variant}` : null;
  useEffect(() => {
    if (!sendKey) return;
    const [stepId, variant] = sendKey.split(":");
    let stopped = false;
    let inFlight = 0;
    let issued = stateRef.current.steps[stepId as StepId].sent;
    let timer: ReturnType<typeof setInterval> | undefined;
    void loadBundle(variant).then((bundle) => {
      if (stopped) return;
      timer = setInterval(() => {
        const s = stateRef.current;
        const p = sending(s);
        if (stopped || !p || s.current !== stepId || inFlight >= MAX_IN_FLIGHT) return;
        // Requests answered so far plus the step's remaining quota bound what may be issued.
        if (issued >= s.steps[stepId as StepId].sent + p.remaining) return;
        const tile = bundle.tiles[issued % bundle.tiles.length];
        issued++;
        inFlight++;
        fetch(`/api/v1/models/${model}/inferences`, {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ image: tile.jpeg, contentType: "image/jpeg" }),
        })
          .then(async (r) => {
            const body = await r.json().catch(() => ({}));
            if (!r.ok) throw new Error(r.status === 502 ? "The platform stopped answering. Sending paused; check that it's running." : (body.message ?? `The gateway answered ${r.status}.`));
            // Recorded even after the step ended: the job exists, and it belongs to this step.
            feed({ kind: "sent", jobId: body.jobId, tile: tile.id, cls: tile.cls, variant, step: stepId as StepId });
          })
          .catch((e: Error) => {
            stopped = true;
            setError(e.message);
          })
          .finally(() => inFlight--);
      }, 1000 / rate);
    });
    return () => {
      stopped = true;
      clearInterval(timer);
    };
  }, [sendKey, rate, model, feed, loadBundle]);

  // Replay: the recording's inputs on its schedule, at a chosen speed.
  const timeline = useMemo(() => (recording ? schedule(recording.entries) : []), [recording]);
  const [speed, setSpeed] = useState(1);
  const [playing, setPlaying] = useState(false);
  const clock = useRef(0);
  const next = useRef(0);
  const [progress, setProgress] = useState({ ms: 0, cursor: 0 });
  useEffect(() => {
    if (live) return;
    for (const v of ["clean", "haze"]) void loadBundle(v);
  }, [live, loadBundle]);
  useEffect(() => {
    if (live || !playing) return;
    let last = performance.now();
    const t = setInterval(() => {
      const now = performance.now();
      clock.current += (now - last) * speed;
      last = now;
      while (next.current < timeline.length && timeline[next.current].at <= clock.current) feed(timeline[next.current++].input);
      setProgress({ ms: clock.current, cursor: next.current });
      if (next.current >= timeline.length) setPlaying(false);
    }, 50);
    return () => clearInterval(t);
  }, [live, playing, speed, timeline, feed]);

  const total = timeline.length ? timeline[timeline.length - 1].at : 0;
  const recent = <RecentTiles state={state} image={image} />;

  return (
    <div className="mt-6 space-y-6">
      {!live && recording && (
        <div className="flex flex-wrap items-center gap-3 rounded-2xl bg-wait-soft p-4 text-sm">
          <button type="button" onClick={() => setPlaying((p) => !p)} disabled={progress.cursor >= timeline.length} className="rounded-full bg-ink px-4 py-2 font-medium text-white hover:brightness-125 disabled:opacity-50">
            {playing ? "Pause" : progress.cursor === 0 ? "Play the recording" : progress.cursor >= timeline.length ? "Finished" : "Resume"}
          </button>
          <label className="flex items-center gap-2">
            Speed
            <select value={speed} onChange={(e) => setSpeed(Number(e.target.value))} className="rounded-lg bg-surface px-2 py-1 ring-1 ring-rule">
              {[1, 2, 4, 8, 16].map((x) => (
                <option key={x} value={x}>
                  {x}×
                </option>
              ))}
            </select>
          </label>
          <span className="text-wait">
            {fmtDuration(progress.ms)} of {fmtDuration(total)} at 1× (the run itself took {fmtDuration(lengths(recording.entries).realMs)}; waits over 20 s are shortened and say so)
          </span>
        </div>
      )}
      <PathStrip state={state} />
      <div className="grid gap-6 lg:grid-cols-[minmax(0,6fr)_minmax(0,5fr)]">
        <Steps state={state} live={live} feed={feed} rate={rate} setRate={setRate} error={error} recent={recent} />
        <div className="space-y-6">
          <DriftMath state={state} />
          <AccuracyPanel state={state} />
          <EventLog state={state} />
        </div>
      </div>
      {live && (
        <p className="text-sm text-muted">
          This run is logged in your browser as it goes.{" "}
          <button type="button" onClick={() => download(record(model, startedAt.current, log.current))} className={link}>
            Save it as a recording
          </button>{" "}
          to replay it later or where the platform isn&apos;t running.
        </p>
      )}
    </div>
  );
}

function record(model: string, recordedAt: string, entries: Entry[]): Recording {
  return {
    recordedAt,
    model,
    about: "Every input a live run of the dashboard's demo page received (events from the gateway's feed, tiles sent, version lists, gate reports, and the person's actions), each with its time in milliseconds since the run began.",
    entries: [...entries],
  };
}

function download(rec: Recording) {
  const blob = new Blob([JSON.stringify(rec)], { type: "application/json" });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = `spotter-demo-${rec.recordedAt.slice(0, 19).replace(/[:T]/g, "-")}.json`;
  a.click();
  URL.revokeObjectURL(a.href);
}
