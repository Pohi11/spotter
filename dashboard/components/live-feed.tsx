"use client";

import { useState } from "react";

import { countByStream, summarize } from "@/lib/events";
import { type FeedState, useEventFeed } from "@/lib/use-event-feed";

import { Chip, Dot } from "./ui";

// The live page's client part (spec: dashboard "The live page shows events as they arrive").

const STREAMS = [
  { name: "INFERENCE", tone: "model", about: "Jobs requested, completed, failed, and shadow comparisons" },
  { name: "REGISTRY", tone: "code", about: "Registrations and stage changes, published from the registry's outbox" },
  { name: "MONITOR", tone: "bus", about: "Drift evaluations and the drift state changes they cause" },
] as const;

const STATE: Record<FeedState, { label: string; tone: "pass" | "wait" | "fail" }> = {
  connecting: { label: "Connecting to the event feed…", tone: "wait" },
  live: { label: "Live: new events appear at the top", tone: "pass" },
  reconnecting: { label: "Connection lost, reconnecting…", tone: "fail" },
};

export function LiveFeed() {
  const { events, state } = useEventFeed("5m");
  const [only, setOnly] = useState<string | null>(null);
  const counts = countByStream(events);
  const shown = only ? events.filter((e) => e.stream === only) : events;
  const s = STATE[state];
  return (
    <>
      <p role="status" className="mt-8 inline-flex items-center gap-2 rounded-full bg-surface px-3 py-1 text-sm ring-1 ring-rule">
        <Dot tone={s.tone} className={state === "live" ? "animate-pulse" : ""} />
        {s.label}
      </p>

      <div className="mt-6 grid gap-3 sm:grid-cols-3">
        {STREAMS.map((st) => {
          const pressed = only === st.name;
          return (
            <button
              key={st.name}
              type="button"
              aria-pressed={pressed}
              onClick={() => setOnly(pressed ? null : st.name)}
              className={`rounded-xl p-4 text-left ring-1 transition-colors ${pressed ? "bg-ink text-white ring-ink" : "bg-surface ring-rule hover:bg-wash"}`}
            >
              <span className="flex items-center gap-2 text-sm font-medium">
                <Dot tone={st.tone} />
                {st.name}
              </span>
              <span className="mt-1 block text-3xl font-semibold tabular-nums" data-testid={`count-${st.name}`}>
                {counts[st.name] ?? 0}
              </span>
              <span className={`mt-1 block text-xs leading-relaxed ${pressed ? "text-white/70" : "text-muted"}`}>{st.about}</span>
            </button>
          );
        })}
      </div>
      <p className="mt-2 text-xs text-faint">Counts cover the events shown here: the last 5 minutes and everything since this page opened. Select a stream to show only its events.</p>

      {shown.length === 0 ? (
        <p className="mt-10 max-w-xl leading-relaxed text-muted">
          No events in the last 5 minutes. Classify a tile on the home page and its request and
          completion appear here.
        </p>
      ) : (
        <ol aria-label="Events, newest first" className="mt-8 divide-y divide-rule border-y border-rule">
          {shown.map((ev) => {
            const sum = summarize(ev);
            return (
              <li key={`${ev.stream}:${ev.sequence}`} className="py-3">
                <details className="group">
                  <summary className="grid cursor-pointer list-none grid-cols-[5.5rem_minmax(0,1fr)] gap-x-4 gap-y-1 sm:grid-cols-[5.5rem_9rem_minmax(0,1fr)]">
                    <time dateTime={ev.time} className="text-sm text-muted tabular-nums">
                      {new Date(ev.time).toLocaleTimeString("en-US", { hour12: false })}
                    </time>
                    <span className="flex items-center gap-2 text-sm font-medium">
                      <Dot tone={sum.tone} />
                      {sum.title}
                    </span>
                    <span className="col-span-2 min-w-0 text-sm text-muted sm:col-span-1">
                      <span className="break-words">{sum.detail}</span>
                      <span className="mt-0.5 block truncate font-mono text-xs text-faint">{ev.subject}</span>
                    </span>
                  </summary>
                  <div className="mt-3 sm:ml-[15.5rem]">
                    <div className="flex flex-wrap gap-2">
                      <Chip tone="plain">{ev.stream} #{ev.sequence}</Chip>
                      <Chip tone="plain">{ev.type || "no type header"}</Chip>
                    </div>
                    <pre className="mt-2 max-h-72 overflow-auto rounded-lg bg-ink p-3 text-xs leading-relaxed text-white/90">
                      {JSON.stringify(ev.payload, null, 2)}
                    </pre>
                  </div>
                </details>
              </li>
            );
          })}
        </ol>
      )}
    </>
  );
}
