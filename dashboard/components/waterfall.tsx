import type { Tone } from "@/lib/events";

import { Dot, fmtMs } from "./ui";

// The recorded job's trace as a waterfall: one row per span, positioned on a shared time
// axis. Span names are translated into what each step does; the code names stay visible.

export interface Span {
  service: string;
  name: string;
  start_ms: number;
  duration_ms: number;
}

function describe(s: Span): { text: string; tone: Tone } {
  const n = s.name;
  if (n === "POST /" || n.endsWith("SubmitInference")) return { text: "Gateway handles the request", tone: "code" };
  if (n.endsWith("GetModel")) {
    return s.service === "registry"
      ? { text: "Registry answers", tone: "code" }
      : { text: "Gateway checks the model exists", tone: "code" };
  }
  if (n === "store image") return { text: "Gateway stores the tile", tone: "code" };
  if (n === "record job") return { text: "Gateway records a pending job", tone: "code" };
  if (n.startsWith("publish inference.requested")) return { text: "Gateway publishes the job", tone: "bus" };
  if (n.startsWith("process")) return { text: "Worker takes the job", tone: "model" };
  if (n === "download image") return { text: "Worker reads the tile", tone: "model" };
  if (n === "classify") return { text: "Worker runs the model", tone: "model" };
  if (n === "record result") return { text: "Worker records the result", tone: "model" };
  if (n.startsWith("publish inference.completed")) return { text: "Worker publishes the completion", tone: "bus" };
  return { text: n, tone: "plain" };
}

const BAR: Record<Tone, string> = {
  code: "bg-code",
  bus: "bg-bus",
  model: "bg-model",
  pass: "bg-pass",
  wait: "bg-wait",
  human: "bg-human",
  fail: "bg-fail",
  plain: "bg-faint",
};

export function Waterfall({ spans }: { spans: Span[] }) {
  // Registry's client and server spans start together; show the client first.
  const rows = [...spans].sort((a, b) => a.start_ms - b.start_ms || (a.service === "registry" ? 1 : -1));
  const end = Math.max(...rows.map((s) => s.start_ms + s.duration_ms));
  return (
    <figure aria-label="The recorded job's trace, span by span">
      <ol className="space-y-1.5">
        {rows.map((s, i) => {
          const d = describe(s);
          return (
            <li key={i} className="grid grid-cols-1 gap-x-4 gap-y-1 text-sm sm:grid-cols-[14.5rem_minmax(0,1fr)] sm:items-center">
              <span className="flex min-w-0 items-center gap-2">
                <Dot tone={d.tone} />
                <span className="truncate">{d.text}</span>
              </span>
              <span className="relative h-6 rounded bg-wash">
                <span
                  className={`absolute inset-y-1 rounded-sm ${BAR[d.tone]}`}
                  style={{ left: `${(s.start_ms / end) * 100}%`, width: `max(3px, ${(s.duration_ms / end) * 100}%)` }}
                />
                <span
                  className="absolute inset-y-0 flex items-center pl-2 text-xs whitespace-nowrap text-muted"
                  style={
                    s.start_ms / end > 0.6
                      ? { right: `${100 - (s.start_ms / end) * 100}%`, paddingRight: "0.5rem" }
                      : { left: `${((s.start_ms + s.duration_ms) / end) * 100}%` }
                  }
                >
                  {fmtMs(s.duration_ms)}
                </span>
              </span>
            </li>
          );
        })}
      </ol>
      <figcaption className="mt-3 flex justify-between text-xs text-faint sm:ml-[15.5rem]">
        <span>0 ms</span>
        <span>{fmtMs(end)}</span>
      </figcaption>
    </figure>
  );
}
