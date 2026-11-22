"use client";

import { CartesianGrid, Legend, Line, LineChart, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

import { DETECT, FEATURES, FEATURE_LABELS, RESOLVE, byVersion, evaluations } from "@/lib/drift";
import { useEventFeed } from "@/lib/use-event-feed";

import { Chip, Dot } from "./ui";

// Live PSI per feature from DriftEvaluated events (spec: dashboard "The drift page charts
// PSI over time"). Reads the last 24 hours of the MONITOR stream, then follows it.

const COLORS: Record<string, string> = {
  mean_r: "#b8322a",
  mean_g: "#2e7d3a",
  mean_b: "#1f5fa8",
  contrast: "#8a6100",
  confidence: "#5b45b8",
};

// PSI spans from about 0.01 (noise) to above 10 (a hazed window), so the axis is
// logarithmic; values below the floor are drawn at the floor.
const FLOOR = 0.005;

export function DriftCharts() {
  const { events, state } = useEventFeed("24h", 5000, "MONITOR");
  const versions = byVersion(evaluations(events));
  if (versions.length === 0) {
    return (
      <p className="mt-8 max-w-xl leading-relaxed text-muted">
        {state === "live"
          ? "No drift evaluations in the last 24 hours. The monitor evaluates a version once its window holds 200 completed jobs, then every 50. Classify tiles on the home page, or run spotterload, to fill it."
          : state === "reconnecting"
            ? "The event feed can't be reached. The charts appear when the platform is running."
            : "Reading the last 24 hours of drift evaluations…"}
      </p>
    );
  }
  return (
    <div className="mt-8 space-y-12">
      {versions.map((v) => {
        const latest = Object.fromEntries(v.latest.features.map((f) => [f.feature, f.psi]));
        const data = v.points.map((p, i) => {
          const row: Record<string, number> = { n: i + 1, t: p.t };
          for (const f of FEATURES) if (p[f] !== undefined) row[f] = Math.max(p[f], FLOOR);
          return row;
        });
        return (
          <figure key={v.versionId} data-version={v.version} data-drifting={v.drifting}>
            <figcaption className="flex flex-wrap items-center gap-3">
              <h3 className="text-xl font-semibold">Version {v.version}</h3>
              {v.drifting ? <Chip tone="fail">drifting</Chip> : <Chip tone="pass">no drift</Chip>}
              <span className="text-sm text-muted">
                {v.points.length} evaluations, latest {new Date(v.latest.evaluatedAt).toLocaleString("en-US", { timeStyle: "medium", dateStyle: "medium" })}
              </span>
            </figcaption>
            <div className="mt-4 h-72 rounded-2xl bg-surface p-3 ring-1 ring-rule">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={data} margin={{ top: 10, right: 16, bottom: 0, left: -8 }}>
                  <CartesianGrid stroke="#e3e9ee" vertical={false} />
                  <XAxis dataKey="n" type="number" domain={[1, "dataMax"]} allowDecimals={false} tick={{ fontSize: 12, fill: "#52606f" }} label={{ value: "evaluation", position: "insideBottomRight", offset: -2, fontSize: 11, fill: "#7d8a99" }} />
                  <YAxis scale="log" domain={[FLOOR, "auto"]} allowDataOverflow ticks={[0.01, 0.1, 0.2, 1, 10]} tick={{ fontSize: 12, fill: "#52606f" }} width={44} />
                  <Tooltip
                    labelFormatter={(n, items) => `Evaluation ${n}, ${new Date(Number(items?.[0]?.payload?.t)).toLocaleTimeString("en-US", { hour12: false })}`}
                    formatter={(val, name) => [Number(val) <= FLOOR ? `below ${FLOOR}` : Number(val).toFixed(3), FEATURE_LABELS[String(name)] ?? name]}
                  />
                  <Legend formatter={(name) => FEATURE_LABELS[String(name)] ?? name} wrapperStyle={{ fontSize: 12 }} />
                  <ReferenceLine y={DETECT} stroke="#b8322a" strokeDasharray="5 4" label={{ value: "raise at 0.2", position: "insideTopRight", fontSize: 11, fill: "#b8322a" }} />
                  <ReferenceLine y={RESOLVE} stroke="#2e7d3a" strokeDasharray="5 4" label={{ value: "clear below 0.1", position: "insideBottomRight", fontSize: 11, fill: "#2e7d3a" }} />
                  {FEATURES.map((f) => (
                    <Line key={f} dataKey={f} stroke={COLORS[f]} dot={false} strokeWidth={2} isAnimationActive={false} />
                  ))}
                </LineChart>
              </ResponsiveContainer>
            </div>
            <dl className="mt-3 flex flex-wrap gap-x-6 gap-y-1 text-sm">
              {FEATURES.map((f) => (
                <div key={f} className="flex items-center gap-2">
                  <Dot tone={latest[f] >= DETECT ? "fail" : "plain"} />
                  <dt className="text-muted">{FEATURE_LABELS[f]}</dt>
                  <dd className="font-medium tabular-nums">{latest[f]?.toFixed(3) ?? "?"}</dd>
                </div>
              ))}
            </dl>
          </figure>
        );
      })}
    </div>
  );
}
