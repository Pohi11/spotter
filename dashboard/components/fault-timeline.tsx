import type { FailureRun } from "@/lib/results";

// One failure-lab run as completions per second, with the fault window shaded and
// rejections drawn below the axis. Server-rendered SVG.

export function FaultTimeline({ run, max }: { run: FailureRun; max: number }) {
  const secs = Math.min(run.report.completed_per_second.length, Math.ceil(run.config.duration_s + 15));
  const w = 240;
  const h = 56;
  const bw = w / secs;
  const y = (v: number) => (v / Math.max(1, max)) * (h - 14);
  const rejMax = Math.max(1, ...run.report.rejected_per_second);
  return (
    <svg viewBox={`0 0 ${w} ${h + 14}`} role="img" aria-label={`Completions per second, ${run.mode} mode, ${run.fault}`} className="h-auto w-full max-w-[16rem]">
      <rect x={run.config.fault_at_s * bw} y={0} width={(run.config.heal_at_s - run.config.fault_at_s) * bw} height={h + 14} className="fill-fail-soft" />
      {run.report.completed_per_second.slice(0, secs).map((v, i) => (
        <rect key={`c${i}`} x={i * bw + 0.5} y={h - y(v)} width={Math.max(0.5, bw - 1)} height={y(v)} className={run.mode === "sync" ? "fill-code" : "fill-bus"} />
      ))}
      {run.report.rejected_per_second.slice(0, secs).map((v, i) =>
        v > 0 ? <rect key={`r${i}`} x={i * bw + 0.5} y={h + 2} width={Math.max(0.5, bw - 1)} height={(v / rejMax) * 10} className="fill-fail" /> : null,
      )}
      <line x1={0} x2={w} y1={h + 1} y2={h + 1} className="stroke-rule" />
    </svg>
  );
}
