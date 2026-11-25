"use client";

import { useState } from "react";

import { Chip, Code, Dot } from "@/components/ui";
import { className } from "@/lib/classes";
import { COMPONENTS, explain } from "@/lib/explain";
import { ENTER, binLabel, breakdown, featureName, worst } from "@/lib/psi";
import { type State, type StepId, accuracy, production, watchedEvaluation } from "@/lib/scenario";
import type { Tone } from "@/lib/events";

// The demo page's "behind the scenes" panels (spec: dashboard "The demo page explains every
// event it sees" and "The demo page shows the arithmetic behind each drift evaluation").

export const STEP_TITLES: Record<StepId, string> = {
  normal: "Normal traffic",
  haze: "Haze rolls in",
  retrain: "Retraining",
  shadow: "Shadow traffic",
  decide: "A person decides",
  after: "Haze again",
};

const TONE: Record<string, Tone> = { gateway: "code", worker: "model", monitor: "bus", registry: "pass", other: "plain" };

export function Panel({ title, intro, children, id }: { title: string; intro?: React.ReactNode; children: React.ReactNode; id?: string }) {
  return (
    <section aria-labelledby={id ? `${id}-h` : undefined} className="rounded-2xl bg-surface p-5 ring-1 ring-rule">
      <h3 id={id ? `${id}-h` : undefined} className="font-semibold">
        {title}
      </h3>
      {intro && <p className="mt-1 text-sm leading-relaxed text-muted">{intro}</p>}
      <div className="mt-4">{children}</div>
    </section>
  );
}

/** The services, lit by the events they publish. */
export function PathStrip({ state }: { state: State }) {
  const latest = state.events[0] ? explain(state.events[0]).publisher : null;
  const retraining = state.steps.retrain.status === "running";
  return (
    <div aria-label="Which service published what" className="rounded-2xl bg-surface p-4 ring-1 ring-rule">
      <ol className="grid gap-2 sm:grid-cols-2 lg:grid-cols-5">
        {COMPONENTS.map((c) => {
          const n = state.published[c.id] ?? 0;
          const lit = latest === c.id;
          return (
            <li key={c.id} data-component={c.id} className={`rounded-xl px-3 py-2.5 ring-1 transition-colors ${lit ? "bg-wash ring-ink" : "ring-rule"}`}>
              <p className="flex items-center gap-2 text-sm font-medium">
                <Dot tone={TONE[c.id]} className={lit ? "animate-pulse" : ""} />
                {c.name}
                <span className="ml-auto font-mono text-xs text-muted" aria-label={`${n} events`}>
                  {n.toLocaleString("en-US")}
                </span>
              </p>
              <p className="mt-0.5 text-xs text-muted">{c.does}</p>
            </li>
          );
        })}
        <li data-component="retrainer" className={`rounded-xl px-3 py-2.5 ring-1 ${retraining ? "bg-human-soft ring-human" : "ring-rule"}`}>
          <p className="flex items-center gap-2 text-sm font-medium">
            <Dot tone="human" className={retraining ? "animate-pulse" : ""} />
            Retrainer
          </p>
          <p className="mt-0.5 text-xs text-muted">consumes drift events; acts through the registry</p>
        </li>
      </ol>
      <p className="mt-3 text-xs text-faint">
        Every number is an event this page received on the gateway&apos;s feed. All of them travel over NATS JetStream, at least once, each with a message id.
      </p>
    </div>
  );
}

export function DriftMath({ state }: { state: State }) {
  const e = watchedEvaluation(state);
  const [chosen, setChosen] = useState<string | null>(null);
  const prod = production(state);
  if (!e) {
    return (
      <Panel id="drift-math" title="Drift arithmetic" intro="What the monitor computed, bin by bin.">
        <p className="text-sm text-muted">
          No evaluation yet. The monitor waits until a version has 200 completions in its window, then compares them with the version&apos;s training reference every 50.
        </p>
      </Panel>
    );
  }
  const top = worst(e.features);
  const feature = e.features.find((f) => f.feature === chosen) ?? top;
  const b = feature ? breakdown(feature) : null;
  const scale = Math.max(1, ...e.features.map((f) => f.psi));
  const maxShare = b ? Math.max(...b.bins.flatMap((x) => [x.reference, x.window])) : 1;
  return (
    <Panel
      id="drift-math"
      title="Drift arithmetic"
      intro={
        <>
          Version {e.version}&apos;s last {e.windowSize} inputs against its training reference
          {prod && prod.id !== e.versionId ? ` (version ${prod.version} is in production now and has no evaluation yet)` : ""}.{" "}
          {e.drifting ? <span className="font-medium text-fail">Drifting.</span> : <span className="font-medium text-pass">Not drifting.</span>}
        </>
      }
    >
      <ul aria-label="PSI per feature" className="space-y-2">
        {e.features.map((f) => (
          <li key={f.feature}>
            <button
              type="button"
              onClick={() => setChosen(f.feature)}
              aria-pressed={feature?.feature === f.feature}
              className={`grid w-full grid-cols-[8.5rem_minmax(0,1fr)_3.5rem] items-center gap-2 rounded-md px-1.5 py-1 text-left text-sm hover:bg-wash ${feature?.feature === f.feature ? "bg-wash ring-1 ring-rule" : ""}`}
            >
              <span className="truncate">{featureName(f.feature)}</span>
              <span className="relative h-2 rounded-full bg-wash ring-1 ring-rule">
                <span className={`absolute inset-y-0 left-0 rounded-full ${f.psi >= ENTER ? "bg-fail" : "bg-bus"}`} style={{ width: `${Math.min(100, (f.psi / scale) * 100)}%` }} />
                <span aria-hidden className="absolute -inset-y-1 w-px bg-ink" style={{ left: `${(ENTER / scale) * 100}%` }} title="0.2 threshold" />
              </span>
              <span className={`text-right font-mono ${f.psi >= ENTER ? "text-fail" : ""}`}>{f.psi.toFixed(3)}</span>
            </button>
          </li>
        ))}
      </ul>
      <p className="mt-2 text-xs text-faint">The black tick is the 0.2 threshold. Identical distributions give about 0.05 from sampling noise alone.</p>

      {feature && (
        <div className="mt-5 border-t border-rule pt-4">
          <p className="text-sm">
            <span className="font-medium">{featureName(feature.feature)}</span>: training mean <span className="font-mono">{feature.referenceMean.toFixed(3)}</span>, window mean{" "}
            <span className="font-mono">{feature.windowMean.toFixed(3)}</span>, KS statistic <span className="font-mono">{feature.ksStatistic.toFixed(2)}</span>.
          </p>
          {b ? (
            <>
              <p className="mt-2 text-sm leading-relaxed text-muted">
                The 10 bins are cut at the deciles of this version&apos;s training values, so training puts 10% in each. PSI adds <Code>(w − r) · ln(w / r)</Code> per bin: any bin that gains or loses share adds to the total.
              </p>
              <table className="mt-3 w-full text-xs">
                <caption className="sr-only">Reference and window share per bin, and each bin&apos;s contribution to PSI</caption>
                <thead>
                  <tr className="text-faint">
                    <th scope="col" className="py-1 pr-2 text-left font-medium">Bin</th>
                    <th scope="col" className="py-1 pr-2 text-left font-medium">Training (r) and window (w)</th>
                    <th scope="col" className="py-1 text-right font-medium">Adds</th>
                  </tr>
                </thead>
                <tbody>
                  {b.bins.map((bin, i) => (
                    <tr key={i} className="border-t border-rule/60">
                      <td className="py-1 pr-2 font-mono whitespace-nowrap text-muted">{binLabel(bin)}</td>
                      <td className="py-1 pr-2">
                        <span className="flex items-center gap-1.5">
                          <span className="h-1.5 rounded-full bg-faint" style={{ width: `${(bin.reference / maxShare) * 45}%` }} />
                          <span className="font-mono text-faint">{(bin.reference * 100).toFixed(0)}%</span>
                        </span>
                        <span className="mt-0.5 flex items-center gap-1.5">
                          <span className={`h-1.5 rounded-full ${bin.window > bin.reference * 1.5 || bin.window < bin.reference / 1.5 ? "bg-fail" : "bg-bus"}`} style={{ width: `${(bin.window / maxShare) * 45}%` }} />
                          <span className="font-mono">{(bin.window * 100).toFixed(bin.window < 0.01 ? 2 : 0)}%</span>
                        </span>
                      </td>
                      <td className="py-1 text-right font-mono">{bin.contribution.toFixed(3)}</td>
                    </tr>
                  ))}
                </tbody>
                <tfoot>
                  <tr className="border-t border-ink">
                    <th scope="row" colSpan={2} className="py-1.5 text-left font-medium">
                      Sum = PSI {feature.psi >= ENTER ? "(at or above 0.2: drifting)" : "(under 0.2)"}
                    </th>
                    <td className="py-1.5 text-right font-mono font-semibold" data-testid="psi-sum">
                      {b.sum.toFixed(3)}
                    </td>
                  </tr>
                </tfoot>
              </table>
            </>
          ) : (
            <p className="mt-2 text-sm text-muted">This evaluation came from a monitor that didn&apos;t publish its bins yet.</p>
          )}
        </div>
      )}
    </Panel>
  );
}

export function AccuracyPanel({ state }: { state: State }) {
  const rows = accuracy(state);
  return (
    <Panel
      id="accuracy"
      title="Accuracy on this page's tiles"
      intro="Known only because the demo knows each tile's true class. Production has no labels, which is why the monitor watches the inputs instead."
    >
      {rows.length === 0 ? (
        <p className="text-sm text-muted">Nothing answered yet.</p>
      ) : (
        <table className="w-full text-sm">
          <thead>
            <tr className="text-xs text-faint">
              <th scope="col" className="py-1 text-left font-medium">Step</th>
              <th scope="col" className="py-1 text-left font-medium">Served by</th>
              <th scope="col" className="py-1 text-right font-medium">Right</th>
              <th scope="col" className="py-1 text-right font-medium">Accuracy</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={`${r.step}-${r.version}`} className="border-t border-rule" data-accuracy={`${r.step}-${r.version}`}>
                <td className="py-1.5">{STEP_TITLES[r.step]}</td>
                <td className="py-1.5">version {r.version}</td>
                <td className="py-1.5 text-right font-mono text-muted">
                  {r.right}/{r.total}
                </td>
                <td className={`py-1.5 text-right font-mono font-medium ${r.right / r.total < 0.8 ? "text-fail" : "text-pass"}`}>{((r.right / r.total) * 100).toFixed(1)}%</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {state.foreign > 0 && <p className="mt-2 text-xs text-faint">{state.foreign} completions from other clients were seen and left out.</p>}
    </Panel>
  );
}

/** The page's latest tiles, as sent, marked right or wrong when answered. */
export function RecentTiles({ state, image }: { state: State; image: (variant: string, tile: string) => string | undefined }) {
  if (state.recent.length === 0) return null;
  return (
    <ol aria-label="Latest tiles sent" className="mt-4 flex flex-wrap gap-1.5">
      {state.recent.map((id) => {
        const j = state.jobs[id];
        const src = image(j.variant, j.tile);
        const answered = j.label !== undefined;
        const right = j.label === j.cls;
        const ring = !answered ? "ring-rule" : right ? "ring-pass" : "ring-fail";
        return (
          <li key={id} title={`${className(j.cls)}${answered ? `: version ${j.version} said ${className(j.label)}` : ": waiting"}`}>
            {/* eslint-disable-next-line @next/next/no-img-element -- the exact bytes sent */}
            {src ? <img src={src} alt="" width={40} height={40} className={`h-10 w-10 rounded ring-2 [image-rendering:pixelated] ${ring}`} /> : <span className={`block h-10 w-10 rounded bg-wash ring-2 ${ring}`} />}
            <span className="sr-only">
              {className(j.cls)}, {answered ? (right ? "answered right" : `answered ${className(j.label)}`) : "waiting"}
            </span>
          </li>
        );
      })}
    </ol>
  );
}

export function EventLog({ state }: { state: State }) {
  const story = state.events.filter((e) => explain(e).story);
  const traffic = state.events.filter((e) => !explain(e).story).slice(0, 6);
  return (
    <Panel id="event-log" title="What just happened" intro="Each event this page received, newest first: who published it, on which subject, and why.">
      <h4 className="text-xs font-medium tracking-wide text-faint uppercase">Registry and monitor</h4>
      {story.length === 0 ? (
        <p className="mt-2 text-sm text-muted">None yet.</p>
      ) : (
        <ol aria-label="Registry and monitor events" className="mt-2 max-h-[28rem] space-y-3 overflow-y-auto pr-1">
          {story.map((ev) => (
            <EventRow key={`${ev.stream}:${ev.sequence}`} ev={ev} />
          ))}
        </ol>
      )}
      <h4 className="mt-5 text-xs font-medium tracking-wide text-faint uppercase">Latest job traffic</h4>
      {traffic.length === 0 ? (
        <p className="mt-2 text-sm text-muted">None yet.</p>
      ) : (
        <ol aria-label="Latest job events" className="mt-2 space-y-3">
          {traffic.map((ev) => (
            <EventRow key={`${ev.stream}:${ev.sequence}`} ev={ev} />
          ))}
        </ol>
      )}
    </Panel>
  );
}

function EventRow({ ev }: { ev: import("@/lib/types").FeedEvent }) {
  const x = explain(ev);
  const component = COMPONENTS.find((c) => c.id === x.publisher)?.name ?? "Unknown";
  return (
    <li className="text-sm" data-event={ev.type}>
      <p className="flex flex-wrap items-center gap-x-2 gap-y-1">
        <Chip tone={TONE[x.publisher]}>{component}</Chip>
        <span className="font-medium">{x.title}</span>
        <time className="ml-auto text-xs text-faint" dateTime={ev.time}>
          {new Date(ev.time).toLocaleTimeString("en-US", { hour12: false })}
        </time>
      </p>
      <p className="mt-1 font-mono text-xs text-faint [overflow-wrap:anywhere]">{ev.subject}</p>
      <p className="mt-0.5 leading-relaxed text-muted">{x.why}</p>
    </li>
  );
}
