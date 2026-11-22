import type { Metadata } from "next";

import { DriftCharts } from "@/components/drift-charts";
import { PageTitle, Section } from "@/components/ui";
import { DEGRADATIONS } from "@/lib/classes";
import { FEATURE_LABELS } from "@/lib/drift";
import { loadResults, reportDate } from "@/lib/results";

export const metadata: Metadata = { title: "Drift" };

export default function DriftPage() {
  const { drift } = loadResults();
  const kinds = [...new Set(drift.conditions.map((c) => c.degradation))];
  const strengths = [...new Set(drift.conditions.map((c) => c.strength))].sort();
  return (
    <>
      <PageTitle title="Drift">
        <p>
          The model never sees labels in production, so Spotter watches what it can measure:
          each tile&apos;s brightness per channel, its contrast, and the model&apos;s confidence. For every
          version, the monitor keeps the last {drift.window} completed jobs and every {drift.evaluate_every} compares
          them with the same features over the version&apos;s validation split, recorded when it was
          trained.
        </p>
        <p className="mt-3 text-base">
          The comparison is the population stability index (PSI) per feature. Drift is raised when
          any feature reaches {drift.psi_enter} and cleared when all fall below {drift.psi_exit}; the gap stops
          one noisy window from flapping the alarm. The Kolmogorov–Smirnov test rides along as
          evidence.
        </p>
      </PageTitle>

      <Section id="live-drift" title="PSI over time" intro="Each line is one feature's PSI at each evaluation, read live from the monitor's DriftEvaluated events. The scale is logarithmic: clean traffic sits near 0.05, a hazed window above 10.">
        <DriftCharts />
      </Section>

      <Section
        id="experiment"
        title="How fast drift is caught"
        intro={
          <>
            Measured on {reportDate(drift.date)} against the running platform: {drift.clean_events.toLocaleString()} clean
            holdout tiles first ({drift.false_alarms} false alarms), then each degradation at each strength until
            the monitor raised drift or {drift.cap.toLocaleString()} degraded jobs had gone through. Each cell is the
            number of degraded jobs it took.
          </>
        }
      >
        <div className="mt-8 max-w-3xl overflow-x-auto">
        <table className="w-full min-w-[36rem] border-t border-ink text-left text-sm">
          <thead>
            <tr className="text-xs text-faint">
              <th scope="col" className="py-3 pr-4 font-medium">Degradation</th>
              {strengths.map((s) => (
                <th key={s} scope="col" className="py-3 pr-4 font-medium">
                  Strength {s}
                </th>
              ))}
              <th scope="col" className="py-3 font-medium">Features that drifted at full strength</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-rule border-t border-rule">
            {kinds.map((k) => (
              <tr key={k}>
                <th scope="row" className="py-3 pr-4 font-semibold">
                  {DEGRADATIONS[k] ?? k}
                </th>
                {strengths.map((s) => {
                  const c = drift.conditions.find((x) => x.degradation === k && x.strength === s)!;
                  return (
                    <td key={s} className={`py-3 pr-4 tabular-nums ${c.detected ? "" : "text-fail"}`}>
                      {c.detected ? c.degraded_events_until_detection : "not detected"}
                    </td>
                  );
                })}
                <td className="py-3 text-muted">
                  {drift.conditions
                    .find((x) => x.degradation === k && x.strength === Math.max(...strengths))!
                    .drifted_features.map((f) => FEATURE_LABELS[f] ?? f)
                    .join(", ") || "none"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        </div>
        <p className="mt-5 max-w-2xl leading-relaxed text-muted">
          Missed within {drift.cap.toLocaleString()} jobs:{" "}
          {drift.conditions
            .filter((c) => !c.detected)
            .map((c) => `${(DEGRADATIONS[c.degradation] ?? c.degradation).toLowerCase()} at strength ${c.strength}`)
            .join(", ") || "none"}
          . Light blur barely changes brightness or contrast, and the model stays confident, so no
          measured feature moves far enough. Stronger blur and noise are caught through the
          model&apos;s confidence rather than the pixels.
        </p>
      </Section>
    </>
  );
}
