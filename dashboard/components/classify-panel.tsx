"use client";

import { useEffect, useRef, useState } from "react";

import { DEGRADATIONS, className } from "@/lib/classes";
import type { Job, Sample } from "@/lib/types";

// The classify panel (spec: dashboard "A visitor can classify a sample tile"). It sends
// the chosen file's bytes to the inference API through the proxy and polls the job.

type Phase =
  | { kind: "idle" }
  | { kind: "sending" }
  | { kind: "waiting"; jobId: string }
  | { kind: "done"; job: Job; ms: number }
  | { kind: "error"; message: string };

const STRENGTHS = [
  { value: 0.5, label: "Half" },
  { value: 1, label: "Full" },
];

async function toBase64(url: string): Promise<string> {
  const bytes = new Uint8Array(await (await fetch(url)).arrayBuffer());
  let s = "";
  for (const b of bytes) s += String.fromCharCode(b);
  return btoa(s);
}

const UNREACHABLE = "The platform can't be reached right now. Check that it's running, then try again.";

export function ClassifyPanel({ samples, model }: { samples: Sample[]; model: string }) {
  const [sampleId, setSampleId] = useState(samples[0]?.id ?? "");
  const [degradation, setDegradation] = useState("none");
  const [strength, setStrength] = useState(1);
  const [phase, setPhase] = useState<Phase>({ kind: "idle" });
  const cancelled = useRef(false);
  useEffect(() => {
    cancelled.current = false; // effects can run twice in development
    return () => void (cancelled.current = true);
  }, []);

  const sample = samples.find((s) => s.id === sampleId) ?? samples[0];
  const variant =
    sample.variants.find((v) => v.degradation === degradation && (degradation === "none" || v.strength === strength)) ??
    sample.variants[0];
  const busy = phase.kind === "sending" || phase.kind === "waiting";

  async function submit() {
    setPhase({ kind: "sending" });
    const started = performance.now();
    try {
      const image = await toBase64(`/samples/${variant.file}`);
      const res = await fetch(`/api/v1/models/${model}/inferences`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ image, contentType: "image/jpeg" }),
      });
      const body = await res.json().catch(() => ({}));
      if (!res.ok) {
        setPhase({ kind: "error", message: res.status === 502 ? UNREACHABLE : (body.message ?? `The gateway answered ${res.status}.`) });
        return;
      }
      const jobId: string = body.jobId;
      setPhase({ kind: "waiting", jobId });
      for (let i = 0; i < 150 && !cancelled.current; i++) {
        const r = await fetch(`/api/v1/inferences/${jobId}`);
        if (!r.ok) throw new Error(String(r.status));
        const { job } = (await r.json()) as { job: Job };
        if (job.status !== "JOB_STATUS_PENDING") {
          setPhase({ kind: "done", job, ms: performance.now() - started });
          return;
        }
        await new Promise((ok) => setTimeout(ok, 100));
      }
      setPhase({ kind: "error", message: `Job ${jobId} is still waiting for a worker after 15 seconds. Is the inference worker running?` });
    } catch {
      setPhase({ kind: "error", message: UNREACHABLE });
    }
  }

  return (
    <div className="grid gap-8 rounded-2xl bg-surface p-5 ring-1 ring-rule sm:p-7 lg:grid-cols-[minmax(0,7fr)_minmax(0,5fr)]">
      <div>
        <fieldset>
          <legend className="font-medium">Tile</legend>
          <div className="mt-3 grid grid-cols-5 gap-2 sm:grid-cols-10 lg:grid-cols-5 xl:grid-cols-10">
            {samples.map((s) => {
              const checked = s.id === sampleId;
              return (
                <label key={s.id} className="group relative cursor-pointer">
                  <input
                    type="radio"
                    name="sample"
                    value={s.id}
                    checked={checked}
                    onChange={() => setSampleId(s.id)}
                    className="peer sr-only"
                  />
                  {/* eslint-disable-next-line @next/next/no-img-element -- 64-pixel tiles, shown pixelated */}
                  <img
                    src={`/samples/${s.variants[0].file}`}
                    alt={className(s.true_class)}
                    width={64}
                    height={64}
                    className={`aspect-square w-full rounded-md [image-rendering:pixelated] ring-offset-2 peer-focus-visible:ring-2 peer-focus-visible:ring-code ${checked ? "ring-2 ring-ink" : "opacity-80 group-hover:opacity-100"}`}
                  />
                </label>
              );
            })}
          </div>
        </fieldset>

        <fieldset className="mt-6">
          <legend className="font-medium">Degradation</legend>
          <p className="mt-1 text-sm text-muted">The same functions and strengths the drift experiment used.</p>
          <div className="mt-3 flex flex-wrap gap-2">
            {Object.entries(DEGRADATIONS).map(([value, label]) => (
              <Toggle key={value} name="degradation" value={value} checked={degradation === value} onChange={() => setDegradation(value)}>
                {label}
              </Toggle>
            ))}
          </div>
          {degradation !== "none" && (
            <div className="mt-3 flex flex-wrap items-center gap-2">
              <span className="text-sm text-muted">Strength</span>
              {STRENGTHS.map((s) => (
                <Toggle key={s.value} name="strength" value={String(s.value)} checked={strength === s.value} onChange={() => setStrength(s.value)}>
                  {s.label}
                </Toggle>
              ))}
            </div>
          )}
        </fieldset>

        <button
          type="button"
          onClick={submit}
          disabled={busy}
          className="mt-8 rounded-full bg-ink px-5 py-2.5 font-medium text-white hover:brightness-125 disabled:opacity-60"
        >
          {busy ? "Classifying…" : "Classify this tile"}
        </button>
      </div>

      <div aria-live="polite">
        {/* eslint-disable-next-line @next/next/no-img-element -- the exact file the panel sends */}
        <img
          src={`/samples/${variant.file}`}
          alt={`The ${className(sample.true_class)} tile${degradation === "none" ? "" : `, ${degradation} at ${strength === 1 ? "full" : "half"} strength`}`}
          width={192}
          height={192}
          className="h-48 w-48 rounded-lg [image-rendering:pixelated]"
        />
        <p className="mt-2 text-sm text-muted">
          True class: <span className="font-medium text-ink">{className(sample.true_class)}</span>
        </p>
        <Result phase={phase} trueClass={sample.true_class} />
      </div>
    </div>
  );
}

function Toggle({ name, value, checked, onChange, children }: { name: string; value: string; checked: boolean; onChange: () => void; children: string }) {
  return (
    <label className="cursor-pointer">
      <input type="radio" name={name} value={value} checked={checked} onChange={onChange} className="peer sr-only" />
      <span className="inline-block rounded-full px-3.5 py-1.5 text-sm ring-1 ring-rule peer-checked:bg-ink peer-checked:text-white peer-checked:ring-ink peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-code hover:bg-wash peer-checked:hover:bg-ink">
        {children}
      </span>
    </label>
  );
}

function Result({ phase, trueClass }: { phase: Phase; trueClass: string }) {
  if (phase.kind === "idle") {
    return <p className="mt-6 text-sm text-muted">Pick a tile and a degradation, then classify it. The result appears here.</p>;
  }
  if (phase.kind === "error") {
    return (
      <p role="alert" className="mt-6 rounded-lg bg-fail-soft px-4 py-3 text-sm text-fail">
        {phase.message}
      </p>
    );
  }
  if (phase.kind === "sending") return <p className="mt-6 text-sm text-muted">Sending the tile to the gateway…</p>;
  if (phase.kind === "waiting") {
    return (
      <div className="mt-6 text-sm">
        <p className="text-muted">
          Job <span className="font-mono text-ink">{phase.jobId}</span> is queued on the bus.
        </p>
        <p className="mt-1 text-wait">Status: pending</p>
      </div>
    );
  }
  const { job, ms } = phase;
  if (job.status === "JOB_STATUS_FAILED" || !job.prediction) {
    return (
      <div className="mt-6 text-sm">
        <p className="text-muted">
          Job <span className="font-mono text-ink">{job.jobId}</span>
        </p>
        <p role="alert" className="mt-2 text-fail">Status: failed. {job.error}</p>
      </div>
    );
  }
  const p = job.prediction;
  const right = p.label === trueClass;
  const runnersUp = [...p.scores].sort((a, b) => b.score - a.score).slice(1, 4);
  return (
    <div className="mt-6 text-sm">
      <p className="text-muted">
        Job <span className="font-mono text-ink">{job.jobId}</span>
      </p>
      <p className="mt-1 text-pass">Status: succeeded, {Math.round(ms)} ms after you clicked</p>
      <p className="mt-4 text-2xl font-semibold">
        {className(p.label)} <span className="text-muted">{(p.confidence * 100).toFixed(1)}%</span>
      </p>
      <p className={`mt-1 ${right ? "text-pass" : "text-fail"}`}>{right ? "Matches the true class." : `The true class is ${className(trueClass)}.`}</p>
      <p className="mt-1 text-muted">Served by version {p.modelVersion}.</p>
      <h4 className="mt-4 font-medium">Next most likely</h4>
      <ul className="mt-2 space-y-1.5">
        {runnersUp.map((s) => (
          <li key={s.label} className="grid grid-cols-[9rem_minmax(0,1fr)_3.5rem] items-center gap-2">
            <span className="truncate text-muted">{className(s.label)}</span>
            <span className="h-1.5 rounded-full bg-wash">
              <span className="block h-1.5 rounded-full bg-model" style={{ width: `${Math.max(1, s.score * 100)}%` }} />
            </span>
            <span className="text-right text-muted">{(s.score * 100).toFixed(1)}%</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
