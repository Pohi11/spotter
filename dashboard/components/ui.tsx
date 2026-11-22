import Link from "next/link";
import type { ReactNode } from "react";

import type { Tone } from "@/lib/events";
import { stageName, stageTone } from "@/lib/model";
import type { Stage } from "@/lib/types";

export const link = "text-code underline decoration-code/30 underline-offset-2 hover:decoration-code";

// Full class names, so Tailwind finds them in the source.
const DOT: Record<Tone, string> = {
  bus: "bg-bus",
  code: "bg-code",
  model: "bg-model",
  pass: "bg-pass",
  wait: "bg-wait",
  human: "bg-human",
  fail: "bg-fail",
  plain: "bg-faint",
};

const CHIP: Record<Tone, string> = {
  bus: "bg-bus-soft text-bus",
  code: "bg-code-soft text-code",
  model: "bg-model-soft text-model",
  pass: "bg-pass-soft text-pass",
  wait: "bg-wait-soft text-wait",
  human: "bg-human-soft text-human",
  fail: "bg-fail-soft text-fail",
  plain: "bg-wash text-muted ring-1 ring-rule",
};

export function Dot({ tone, className = "" }: { tone: Tone; className?: string }) {
  return <span aria-hidden className={`inline-block h-2.5 w-2.5 shrink-0 rounded-full ${DOT[tone]} ${className}`} />;
}

export function Chip({ tone, children }: { tone: Tone; children: ReactNode }) {
  return <span className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium whitespace-nowrap ${CHIP[tone]}`}>{children}</span>;
}

export function StageChip({ stage }: { stage: Stage }) {
  return <Chip tone={stageTone(stage)}>{stageName(stage)}</Chip>;
}

export function Code({ children }: { children: ReactNode }) {
  return <code className="rounded bg-wash px-1 py-0.5 font-mono text-[0.85em] text-ink ring-1 ring-rule [overflow-wrap:anywhere]">{children}</code>;
}

export function PageTitle({ title, children }: { title: string; children: ReactNode }) {
  return (
    <header className="max-w-2xl">
      <h1 className="text-4xl font-semibold sm:text-5xl">{title}</h1>
      <div className="mt-4 text-lg leading-relaxed text-muted">{children}</div>
    </header>
  );
}

export function Section({ id, title, intro, children }: { id: string; title: string; intro?: ReactNode; children: ReactNode }) {
  return (
    <section id={id} aria-labelledby={`${id}-heading`} className="mt-24 scroll-mt-6">
      <h2 id={`${id}-heading`} className="text-3xl font-semibold">
        {title}
      </h2>
      {intro && <p className="mt-3 max-w-2xl leading-relaxed text-muted">{intro}</p>}
      {children}
    </section>
  );
}

export function Points({ items, className = "" }: { items: ReactNode[]; className?: string }) {
  return (
    <ul className={`space-y-4 leading-relaxed text-muted ${className}`}>
      {items.map((item, i) => (
        <li key={i} className="flex gap-3">
          <span aria-hidden className="mt-2.5 h-1.5 w-1.5 shrink-0 rounded-full bg-faint" />
          <span>{item}</span>
        </li>
      ))}
    </ul>
  );
}

/** A measured number with its label and, optionally, how it was measured. */
export function Figure({ label, value, note }: { label: string; value: ReactNode; note?: ReactNode }) {
  return (
    <div className="border-t border-ink pt-3">
      <dt className="text-sm text-muted">{label}</dt>
      <dd className="mt-1 text-3xl font-semibold tracking-tight">{value}</dd>
      {note && <dd className="mt-1 text-sm text-faint">{note}</dd>}
    </div>
  );
}

/** What a page shows when the platform can't be reached from the dashboard's server. */
export function Unreachable({ what }: { what: string }) {
  return (
    <div role="status" className="mt-8 max-w-2xl rounded-2xl bg-fail-soft p-5 text-fail">
      <p className="font-medium">The platform can&apos;t be reached, so {what} can&apos;t be shown.</p>
      <p className="mt-1 text-sm text-ink/80">
        Start the platform (see the README&apos;s &ldquo;Run it&rdquo; section) and reload this page. The{" "}
        <Link href="/results" className={link}>results</Link> and{" "}
        <Link href="/architecture" className={link}>architecture</Link> pages work without it.
      </p>
    </div>
  );
}

export function fmtMs(ms: number): string {
  if (ms >= 1000) return `${(ms / 1000).toFixed(ms >= 10000 ? 0 : 1)} s`;
  if (ms >= 100) return `${Math.round(ms)} ms`;
  return `${ms.toFixed(1)} ms`;
}

export const pct = (x: number, digits = 1) => `${(x * 100).toFixed(digits)}%`;
