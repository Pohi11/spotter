"use client";

import { useRouter } from "next/navigation";
import { type FormEvent, useState } from "react";

import { canPromote } from "@/lib/model";
import type { Gate } from "@/lib/types";

import { Dot } from "./ui";

// The model page's actions (spec: dashboard "Promotion is shown gate by gate and needs a
// person" and "Rollback names what it will restore and asks first"). Every action goes to
// the registry through the proxy; the page shows the registry's answer, refusals included.

type Outcome = { ok: boolean; text: string } | null;

async function post(path: string, body: object): Promise<{ ok: boolean; message: string }> {
  try {
    const res = await fetch(`/api/${path}`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body) });
    const json = await res.json().catch(() => ({}));
    if (res.ok) return { ok: true, message: "" };
    return { ok: false, message: json.message ?? `The registry answered ${res.status}.` };
  } catch {
    return { ok: false, message: "The platform can't be reached right now." };
  }
}

function Message({ outcome }: { outcome: Outcome }) {
  if (!outcome) return null;
  return (
    <p role={outcome.ok ? "status" : "alert"} className={`mt-3 rounded-lg px-3 py-2 text-sm ${outcome.ok ? "bg-pass-soft text-pass" : "bg-fail-soft text-fail"}`}>
      {outcome.text}
    </p>
  );
}

const GATE_NAMES: Record<string, string> = {
  accuracy: "Holdout accuracy",
  shadow_agreement: "Agreement with production",
  shadow_latency: "Shadow latency",
  approval: "Human approval",
};

export function Promotion({
  versionId,
  version,
  gates,
  onApproved,
}: {
  versionId: string;
  version: number;
  gates: Gate[];
  /** Told after the registry accepted a person's approval (the demo page records it). */
  onApproved?: (approver: string, reason: string) => void;
}) {
  const router = useRouter();
  const [approver, setApprover] = useState("");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [outcome, setOutcome] = useState<Outcome>(null);
  const promotable = canPromote(gates);

  async function approve(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    const r = await post(`v1/versions/${versionId}:approve`, { approver, reason });
    setOutcome(r.ok ? { ok: true, text: `Approved by ${approver}. The approval gate now reads the recorded approval.` } : { ok: false, text: `Approval refused: ${r.message}` });
    setBusy(false);
    if (r.ok) {
      onApproved?.(approver, reason);
      router.refresh();
    }
  }

  async function promote() {
    setBusy(true);
    const r = await post(`v1/versions/${versionId}:promote`, { reason: reason || `promoted from the dashboard by ${approver || "an operator"}` });
    setOutcome(r.ok ? { ok: true, text: `Version ${version} is in production. Workers load it when the stage-change event reaches them.` } : { ok: false, text: `Promotion refused: ${r.message}` });
    setBusy(false);
    if (r.ok) router.refresh();
  }

  return (
    <div>
      <ul aria-label={`Promotion gates for version ${version}`} className="divide-y divide-rule border-y border-rule">
        {gates.map((g) => (
          <li key={g.name} className="grid grid-cols-[1.25rem_minmax(0,1fr)] gap-x-2 py-3" data-gate={g.name} data-passed={g.passed}>
            <Dot tone={g.passed ? "pass" : g.name === "approval" ? "human" : "fail"} className="mt-1.5" />
            <div>
              <p className="font-medium">
                {GATE_NAMES[g.name] ?? g.name} <span className={`text-sm font-normal ${g.passed ? "text-pass" : "text-fail"}`}>{g.passed ? "passed" : "not passed"}</span>
              </p>
              <p className="text-sm text-muted">{g.detail}</p>
            </div>
          </li>
        ))}
      </ul>

      <form onSubmit={approve} className="mt-6 grid gap-3 sm:grid-cols-2">
        <label className="text-sm">
          <span className="font-medium">Your name</span>
          <input
            required
            value={approver}
            onChange={(e) => setApprover(e.target.value)}
            autoComplete="name"
            className="mt-1 block w-full rounded-lg bg-surface px-3 py-2 ring-1 ring-rule focus:ring-2 focus:ring-code focus:outline-none"
          />
        </label>
        <label className="text-sm">
          <span className="font-medium">Reason</span>
          <input
            required
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="What you checked before approving"
            className="mt-1 block w-full rounded-lg bg-surface px-3 py-2 ring-1 ring-rule focus:ring-2 focus:ring-code focus:outline-none"
          />
        </label>
        <div className="flex flex-wrap items-center gap-3 sm:col-span-2">
          <button type="submit" disabled={busy} className="rounded-full bg-human px-4 py-2 text-sm font-medium text-white hover:brightness-110 disabled:opacity-60">
            Approve version {version}
          </button>
          <button
            type="button"
            onClick={promote}
            disabled={busy || !promotable}
            className="rounded-full bg-pass px-4 py-2 text-sm font-medium text-white hover:brightness-110 disabled:cursor-not-allowed disabled:bg-faint"
          >
            Promote version {version}
          </button>
          {!promotable && <span className="text-sm text-muted">Promote is available once every gate passes.</span>}
        </div>
      </form>
      <Message outcome={outcome} />
    </div>
  );
}

export function Rollback({ model, current, target }: { model: string; current: number; target: number }) {
  const router = useRouter();
  const [confirming, setConfirming] = useState(false);
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [outcome, setOutcome] = useState<Outcome>(null);

  async function rollback(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    const r = await post(`v1/models/${model}:rollback`, { reason });
    setOutcome(r.ok ? { ok: true, text: `Rolled back. Version ${target} is in production again and version ${current} is marked rolled back.` } : { ok: false, text: `Rollback refused: ${r.message}` });
    setBusy(false);
    setConfirming(false);
    if (r.ok) router.refresh();
  }

  return (
    <div>
      <p className="leading-relaxed text-muted">
        A rollback marks version {current} rolled back and puts version {target}, the one most
        recently retired from production, back in production, in one database transaction.
      </p>
      {!confirming ? (
        <button type="button" onClick={() => setConfirming(true)} className="mt-4 rounded-full px-4 py-2 text-sm font-medium text-fail ring-1 ring-fail hover:bg-fail-soft">
          Roll back to version {target}
        </button>
      ) : (
        <form onSubmit={rollback} className="mt-4 rounded-xl bg-fail-soft p-4">
          <p className="font-medium text-fail">
            Roll back from version {current} to version {target}?
          </p>
          <label className="mt-3 block text-sm">
            <span className="font-medium">Reason</span>
            <input
              required
              autoFocus
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              className="mt-1 block w-full rounded-lg bg-surface px-3 py-2 ring-1 ring-rule focus:ring-2 focus:ring-code focus:outline-none"
            />
          </label>
          <div className="mt-3 flex flex-wrap gap-3">
            <button type="submit" disabled={busy} className="rounded-full bg-fail px-4 py-2 text-sm font-medium text-white hover:brightness-110 disabled:opacity-60">
              Confirm rollback
            </button>
            <button type="button" onClick={() => setConfirming(false)} className="rounded-full px-4 py-2 text-sm font-medium ring-1 ring-rule hover:bg-surface">
              Keep version {current}
            </button>
          </div>
        </form>
      )}
      <Message outcome={outcome} />
    </div>
  );
}
