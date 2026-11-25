"use client";

import { useEffect, useState } from "react";

import { Promotion } from "@/components/model-actions";
import { Chip, Code, Dot } from "@/components/ui";
import { fmtDuration } from "@/lib/replay";
import { type Input, SENDS, STEP_ORDER, type State, type StepId, type StepStatus, production, shadowCandidates } from "@/lib/scenario";

import { STEP_TITLES } from "./panels";

// The scenario's steps (spec: dashboard "The demo page walks through the loop step by step").
// Each says what it sends, what to watch for, and finishes on the platform's evidence.

const COPY: Record<StepId, { what: string; watch: string }> = {
  normal: {
    what: "A sensor sends ordinary tiles: 300 EuroSAT holdout tiles the model never trained on, in their natural class mix.",
    watch:
      "Each tile becomes a job: the gateway's request and the worker's completion. The monitor keeps the last 200 completions per version, and every 50 it compares their brightness, contrast, and the model's confidence with what that version saw in training. On normal traffic every PSI stays well under 0.2.",
  },
  haze: {
    what: "The same tiles now arrive hazed, as if seen through thin cloud or a dirty lens.",
    watch:
      "Accuracy falls at once, but production can't see that: it has no labels. It can see the inputs. Brightness rises and contrast falls, each evaluation's window holds more hazed tiles, and when any feature's PSI reaches 0.2 the monitor publishes DriftDetected, once. Sending stops there.",
  },
  retrain: {
    what: "Nobody starts this step. The retrainer is subscribed to drift events.",
    watch:
      "It fine-tunes the newest PyTorch version with haze, blur, noise, and darkening added to the training tiles, measures it on the clean and degraded holdout, registers it, validates it against the 0.85 accuracy floor, and moves it to shadow. That is as far as it is allowed to go. On the laptop where it was measured this took about 7 minutes.",
  },
  shadow: {
    what: "Clean tiles again, now with the new version running in shadow next to production.",
    watch:
      "Only production answers. After each job the worker also runs the shadow version and publishes ShadowCompared; the registry counts one comparison per job. The agreement gate needs 200 comparisons and 95% agreement. Clean tiles, because this gate checks that the new version still does what production already did well.",
  },
  decide: {
    what: "Code checks the gates. A person makes the call.",
    watch:
      "Every gate is plain code with its reason shown. The approval gate needs a recorded name and reason, and promotion is a separate action. Promote retires the old version and promotes the new one in one transaction; the worker swaps models when the stage-change event reaches it.",
  },
  after: {
    what: "The first 100 hazed tiles from step 2, now answered by the new version.",
    watch:
      "Compare the accuracy with step 2 on the same tiles. The monitor would still flag hazed inputs against the new version's own reference, because they really do differ from training: drift says the inputs changed, not that the model is wrong. This step sends 100 tiles, fewer than the 200 a new version's window needs, so no evaluation runs.",
  },
};

const STATUS: Record<StepStatus, { label: string; tone: "plain" | "code" | "wait" | "pass" }> = {
  pending: { label: "later", tone: "plain" },
  ready: { label: "ready", tone: "code" },
  running: { label: "in progress", tone: "wait" },
  done: { label: "done", tone: "pass" },
  skipped: { label: "skipped", tone: "plain" },
};

export interface StepsProps {
  state: State;
  live: boolean;
  feed: (input: Input) => void;
  rate: number;
  setRate: (n: number) => void;
  error: string | null;
  recent: React.ReactNode;
}

export function Steps(props: StepsProps) {
  const { state, live, feed } = props;
  const later = STEP_ORDER.slice(STEP_ORDER.indexOf(state.current as StepId) + 1);
  const candidates = shadowCandidates(state);
  return (
    <div>
      {live && state.current !== "end" && later.length > 0 && (
        <SkipAhead later={later} candidate={candidates[0]?.id} feed={feed} />
      )}
      <ol aria-label="Demo steps" className="space-y-4">
        {STEP_ORDER.map((id, i) => (
          <StepCard key={id} id={id} n={i + 1} {...props} />
        ))}
      </ol>
      {state.current === "end" && (
        <p role="status" className="mt-6 rounded-2xl bg-pass-soft p-5 text-pass">
          That&apos;s the loop: inputs shifted, the monitor noticed without labels, a new version was trained and shadowed without anyone starting it, a person approved it, and it now serves. Roll it back from the Model page if you like; the rollback names what it restores and asks first.
        </p>
      )}
    </div>
  );
}

function SkipAhead({ later, candidate, feed }: { later: StepId[]; candidate?: string; feed: (i: Input) => void }) {
  const [to, setTo] = useState<StepId | "">("");
  const needsCandidate = (s: StepId) => s === "shadow" || s === "decide";
  return (
    <form
      className="mb-4 flex flex-wrap items-center gap-2 text-sm text-muted"
      onSubmit={(e) => {
        e.preventDefault();
        if (to) feed({ kind: "start", step: to, candidateId: needsCandidate(to) ? candidate : undefined });
        setTo("");
      }}
    >
      <label htmlFor="skip-to">Presenting a part of it? Skip to</label>
      <select id="skip-to" value={to} onChange={(e) => setTo(e.target.value as StepId)} className="rounded-lg bg-surface px-2 py-1 ring-1 ring-rule">
        <option value="">choose a step</option>
        {later.map((s) => (
          <option key={s} value={s} disabled={needsCandidate(s) && !candidate}>
            {STEP_TITLES[s]}
            {needsCandidate(s) && !candidate ? " (no version in shadow)" : ""}
          </option>
        ))}
      </select>
      <button type="submit" disabled={!to} className="rounded-full px-3 py-1 ring-1 ring-rule hover:bg-surface disabled:opacity-50">
        Skip
      </button>
    </form>
  );
}

function StepCard({ id, n, state, live, feed, rate, setRate, error, recent }: StepsProps & { id: StepId; n: number }) {
  const st = state.steps[id];
  const current = state.current === id;
  const plan = SENDS[id];
  const copy = COPY[id];
  const status = STATUS[st.status];
  const open = current || st.status === "done";
  return (
    <li
      data-step={id}
      data-status={st.status}
      aria-current={current ? "step" : undefined}
      className={`rounded-2xl p-5 ring-1 ${current ? "bg-surface ring-ink" : "ring-rule"} ${st.status === "pending" || st.status === "skipped" ? "opacity-70" : ""}`}
    >
      <h3 className="flex flex-wrap items-center gap-2 text-lg font-semibold">
        <span className="font-mono text-sm text-faint">{n}</span>
        {STEP_TITLES[id]}
        <span className="ml-auto">
          <Chip tone={status.tone}>{status.label}</Chip>
        </span>
      </h3>
      <p className="mt-2 leading-relaxed">{copy.what}</p>
      {open && <p className="mt-2 text-sm leading-relaxed text-muted">{copy.watch}</p>}

      {plan && (st.status === "running" || st.status === "done" || (current && st.sent > 0)) && (
        <p className="mt-3 text-sm" data-testid={`progress-${id}`}>
          <span className="font-mono">{st.sent}</span> {plan.variant === "clean" ? "clean" : "hazed"} tiles sent, <span className="font-mono">{st.completed}</span> answered
          {id === "after" ? ` of ${plan.max}` : ""}
          {st.status === "running" && st.sent >= plan.max ? ". Sent all it will send; waiting for the evidence." : "."}
        </p>
      )}
      {current && plan && recent}

      {!live && state.skips[id] !== undefined && (
        <p className="mt-3 rounded-lg bg-wait-soft px-3 py-2 text-sm text-wait" data-testid={`skipped-${id}`}>
          Replay shortened: {fmtDuration(state.skips[id]!)} of waiting in this step in the recording plays here in a few seconds.
        </p>
      )}
      {current && st.note && <p className="mt-3 rounded-lg bg-wait-soft px-3 py-2 text-sm text-wait">{st.note}</p>}
      {current && error && (
        <p role="alert" className="mt-3 rounded-lg bg-fail-soft px-3 py-2 text-sm text-fail">
          {error}
        </p>
      )}

      {live && current && st.status === "ready" && plan && (
        <div className="mt-4 flex flex-wrap items-center gap-3">
          <button type="button" onClick={() => feed({ kind: "start", step: id })} className="rounded-full bg-ink px-5 py-2.5 font-medium text-white hover:brightness-125">
            {id === "normal" ? "Start sending clean tiles" : id === "haze" ? "Start sending hazed tiles" : id === "shadow" ? "Start shadow traffic" : "Send the hazed tiles again"}
          </button>
          <Rate rate={rate} setRate={setRate} />
        </div>
      )}
      {live && current && st.status === "running" && plan && (
        <div className="mt-3">
          <Rate rate={rate} setRate={setRate} />
        </div>
      )}

      {current && id === "retrain" && <RetrainWait state={state} live={live} feed={feed} />}
      {current && id === "shadow" && state.candidate && <GateList state={state} />}
      {current && id === "decide" && state.candidate && (
        <div className="mt-4">
          {live ? (
            <Promotion
              versionId={state.candidate.id}
              version={state.candidate.version}
              gates={state.gates ?? []}
              onApproved={(approver, reason) => feed({ kind: "approved", versionId: state.candidate!.id, approver, reason })}
            />
          ) : (
            <>
              <GateList state={state} />
              <p className="mt-3 rounded-lg bg-human-soft px-3 py-2 text-sm text-human" data-testid="recorded-approval">
                {state.approval
                  ? `In the recording, ${state.approval.approver} approved version ${state.candidate.version}: “${state.approval.reason}”. Then they promoted it.`
                  : "In the recording, a person is reviewing the gates. The approval and the promotion were theirs; this replay only shows them."}
              </p>
            </>
          )}
        </div>
      )}

      {st.result && (
        <p className="mt-3 flex gap-2 text-sm text-pass">
          <Dot tone="pass" className="mt-1.5" />
          <span>{st.result}</span>
        </p>
      )}
    </li>
  );
}

function Rate({ rate, setRate }: { rate: number; setRate: (n: number) => void }) {
  return (
    <label className="flex items-center gap-2 text-sm text-muted">
      Pace
      <select value={rate} onChange={(e) => setRate(Number(e.target.value))} className="rounded-lg bg-surface px-2 py-1 ring-1 ring-rule">
        {[2, 5, 10, 20].map((r) => (
          <option key={r} value={r}>
            {r} tiles a second
          </option>
        ))}
      </select>
    </label>
  );
}

function RetrainWait({ state, live, feed }: { state: State; live: boolean; feed: (i: Input) => void }) {
  // Mounted when the step becomes current: that is when the wait began.
  const [waitingSince] = useState(() => Date.now());
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!live) return;
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [live]);
  const prod = production(state);
  const registered = state.versions.filter((v) => prod && v.version > prod.version && v.stage !== "MODEL_VERSION_STAGE_SHADOW");
  const ready = shadowCandidates(state);
  return (
    <div className="mt-4 space-y-3 text-sm">
      {live && (
        <p className="text-muted">
          Waiting for the registry to announce a new version in shadow: <span className="font-mono text-ink">{fmtDuration(now - waitingSince)}</span> so far.
        </p>
      )}
      {registered.length > 0 && (
        <p className="text-muted">
          Seen so far: {registered.map((v) => `version ${v.version} (${v.stage.replace("MODEL_VERSION_STAGE_", "").toLowerCase()})`).join(", ")}.
        </p>
      )}
      {live && (
        <>
          <p className="text-muted">
            Nothing happening? The retrainer is optional. In the Compose demo, start it with <Code>docker compose --env-file demo.env --profile retrain up -d</Code>; it picks up the drift event it missed, because its consumer is durable.
          </p>
          {ready.map((v) => (
            <button key={v.id} type="button" onClick={() => feed({ kind: "start", step: "shadow", candidateId: v.id })} className="rounded-full px-4 py-2 text-sm font-medium ring-1 ring-rule hover:bg-wash">
              Use version {v.version}, already in shadow (trained earlier)
            </button>
          ))}
        </>
      )}
    </div>
  );
}

const GATE_NAMES: Record<string, string> = {
  accuracy: "Holdout accuracy",
  shadow_agreement: "Agreement with production",
  shadow_latency: "Shadow latency",
  approval: "Human approval",
};

function GateList({ state }: { state: State }) {
  const gates = state.gates;
  if (!gates) return <p className="mt-4 text-sm text-muted">Asking the registry for version {state.candidate?.version}&apos;s gates…</p>;
  return (
    <ul aria-label={`Gates for version ${state.candidate?.version}`} className="mt-4 divide-y divide-rule border-y border-rule">
      {gates.map((g) => (
        <li key={g.name} className="grid grid-cols-[1.25rem_minmax(0,1fr)] gap-x-2 py-2 text-sm" data-gate={g.name} data-passed={g.passed}>
          <Dot tone={g.passed ? "pass" : g.name === "approval" ? "human" : "fail"} className="mt-1.5" />
          <span>
            <span className="font-medium">{GATE_NAMES[g.name] ?? g.name}</span> <span className={g.passed ? "text-pass" : "text-fail"}>{g.passed ? "passed" : "not passed"}</span>
            <span className="block text-muted">{g.detail}</span>
          </span>
        </li>
      ))}
    </ul>
  );
}
