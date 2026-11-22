import type { Metadata } from "next";
import Link from "next/link";
import type { ReactNode } from "react";

import { FaultTimeline } from "@/components/fault-timeline";
import { Code, Figure, PageTitle, Section, fmtMs, link, pct } from "@/components/ui";
import { type FailureRun, LOAD_RUNS, loadResults, reportDate, runKey } from "@/lib/results";

// The results page (spec: dashboard "Results come from dated reports"). Numbers come from
// benchmarks/*.json; the words around them say what each run was and what it showed.

export const metadata: Metadata = { title: "Results" };

const REPO = "https://github.com/Pohi11/spotter/blob/main/benchmarks/";

function Source({ file, children }: { file: string; children?: ReactNode }) {
  return (
    <p className="mt-3 text-sm text-faint">
      {children} Source:{" "}
      <a href={`${REPO}${file}`} className={link} target="_blank" rel="noopener noreferrer">
        benchmarks/{file}
      </a>
    </p>
  );
}

const VARIANT_NAMES: Record<string, string> = {
  "pytorch-fp32": "PyTorch, fp32 (version 1)",
  "onnx-fp32": "ONNX Runtime, fp32 (version 2)",
  "onnx-int8": "ONNX Runtime, int8 (version 3)",
};

const th = "py-3 pr-4 font-medium";
const td = "py-3 pr-4 tabular-nums";

export default function ResultsPage() {
  const r = loadResults();
  const v = r.variants;
  return (
    <>
      <PageTitle title="Results">
        <p>
          Every measurement Spotter reports, read from the dated JSON files in the repository&apos;s{" "}
          <Code>benchmarks/</Code> folder, which the measuring code wrote. Nothing on this page is
          typed by hand except what each run was. Runs were on one laptop (Windows 11, 22 logical
          cores) unless the section says otherwise, so the absolute numbers are this machine&apos;s; the
          comparisons are the point.
        </p>
      </PageTitle>

      <Section
        id="variants"
        title="The model and its optimized variants"
        intro={`The same trained network exported to ONNX and quantized to int8 (static, per channel, calibrated on 512 training tiles). Accuracy is on all ${v.holdout_tiles.toLocaleString()} holdout tiles; latency is the model alone with ${v.environment.threads} threads.`}
      >
        <div className="mt-8 overflow-x-auto">
          <table className="w-full min-w-[40rem] border-t border-ink text-left text-sm">
            <thead>
              <tr className="text-xs text-faint">
                <th scope="col" className={th}>Variant</th>
                <th scope="col" className={th}>Size</th>
                <th scope="col" className={th}>Holdout accuracy</th>
                <th scope="col" className={th}>Macro F1</th>
                <th scope="col" className={th}>p50, batch of 1</th>
                <th scope="col" className={th}>Images/s, batch of 32</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-rule border-t border-rule">
              {v.variants.map((x) => (
                <tr key={x.name} data-variant={x.name}>
                  <th scope="row" className="py-3 pr-4 font-semibold">{VARIANT_NAMES[x.name] ?? x.name}</th>
                  <td className={td}>{Math.round(x.size_bytes / 1024).toLocaleString()} KB</td>
                  <td className={td}>{pct(x.accuracy, 2)}</td>
                  <td className={td}>{x.macro_f1.toFixed(4)}</td>
                  <td className={td}>{fmtMs(x.latency.find((l) => l.batch === 1)!.p50_ms)}</td>
                  <td className={td}>{Math.round(x.latency.find((l) => l.batch === 32)!.images_per_s).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <Source file="2026-11-22-model-variants.json">
          Measured {reportDate(v.date)}. ONNX fp32 matches PyTorch to {v.parity.max_abs_diff.toExponential(2)} in probability, with{" "}
          {v.parity.disagreements} disagreeing predictions.
        </Source>
      </Section>

      <Section
        id="load"
        title="End to end through the platform"
        intro={
          <>
            <Code>spotterload</Code> (Go) submits holdout tiles to the gateway and times each job from
            submission to its completion event on the bus. Closed loop means each client waits
            for its result before sending the next. The model is a few milliseconds of each job;
            the rest is the path around it.
          </>
        }
      >
        <div className="mt-8 overflow-x-auto">
          <table className="w-full min-w-[48rem] border-t border-ink text-left text-sm">
            <thead>
              <tr className="text-xs text-faint">
                <th scope="col" className={th}>Run</th>
                <th scope="col" className={th}>Clients</th>
                <th scope="col" className={th}>Jobs</th>
                <th scope="col" className={th}>Jobs/s</th>
                <th scope="col" className={th}>p50</th>
                <th scope="col" className={th}>p95</th>
                <th scope="col" className={th}>p99</th>
                <th scope="col" className={th}>Lost</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-rule border-t border-rule">
              {r.load.map((l) => (
                <tr key={l.file}>
                  <th scope="row" className="py-3 pr-4 font-medium">
                    {LOAD_RUNS[runKey(l.file)]}
                    <span className="block text-xs font-normal text-faint">{reportDate(l.date)}</span>
                  </th>
                  <td className={td}>{l.concurrency}{l.closed_loop ? "" : " (open)"}</td>
                  <td className={td}>{l.report.submitted.toLocaleString()}</td>
                  <td className={td}>{l.report.completed_per_second.toFixed(1)}</td>
                  <td className={td}>{fmtMs(l.report.p50_ms)}</td>
                  <td className={td}>{fmtMs(l.report.p95_ms)}</td>
                  <td className={td}>{fmtMs(l.report.p99_ms)}</td>
                  <td className={`${td} ${l.report.lost ? "text-fail" : ""}`}>{l.report.lost.toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <ul className="mt-6 max-w-3xl space-y-3 leading-relaxed text-muted">
          <li>
            <span className="font-medium text-ink">The model isn&apos;t the bottleneck.</span> PyTorch fp32 and
            int8 deliver about the same jobs per second end to end. Traces show writing each tile
            to the object store takes most of the submit call; the cause is still open.
          </li>
          <li>
            <span className="font-medium text-ink">Open loop past capacity builds a queue, not losses.</span>{" "}
            Clients submitted about 150 jobs/s against one worker&apos;s 64, so latency grew to tens of
            seconds. The &ldquo;lost&rdquo; jobs were still queued when the load generator stopped
            waiting, and completed afterwards.
          </li>
          <li>
            <span className="font-medium text-ink">A shadow model once cut throughput to 7.5 jobs/s.</span> ONNX
            Runtime&apos;s thread pools busy-wait; with two sessions loaded the worker used 16 cores.
            One thread per model fixed it (43 jobs/s at 0.49 cores).
          </li>
          <li>
            <span className="font-medium text-ink">Fetching eight messages per pull hurt the tail</span> with two
            workers (p99 973 ms against 169 ms for one per pull), so the worker fetches one.
          </li>
        </ul>
        <Source file="2026-11-22-load-tests.md">Each row is one JSON report next to the write-up.</Source>
      </Section>

      <Section
        id="promotion"
        title="Shadow, promotion, and rollback"
        intro={`Version ${r.promotion.shadow.shadow_version} ran in shadow next to production version ${r.promotion.shadow.production_version}, then was promoted after approval and rolled back, on the running platform.`}
      >
        <dl className="mt-10 grid grid-cols-1 gap-x-8 gap-y-8 sm:grid-cols-2 lg:grid-cols-4">
          <Figure label="Shadow agreement" value={pct(r.promotion.shadow.agreeing / r.promotion.shadow.comparisons)} note={`${r.promotion.shadow.agreeing} of ${r.promotion.shadow.comparisons} jobs`} />
          <Figure label="Promotion to the worker serving it" value={fmtMs(r.promotion.promote_to_serving_ms)} note="commit to the worker's reload" />
          <Figure label="Rollback to the worker serving it" value={fmtMs(r.promotion.rollback_to_serving_ms)} note="commit to the worker's reload" />
          <Figure label="Rollback under load" value={`${r.promotion.rollback_under_load.lost} lost`} note={`${r.promotion.rollback_under_load.submitted} jobs, ${r.promotion.rollback_under_load.failed} failed`} />
        </dl>
        <Source file="2026-11-22-promotion-run.json">{r.promotion.method}.</Source>
      </Section>

      <Section
        id="kubernetes"
        title="A load spike on Kubernetes"
        intro="The whole platform installed from the Helm chart into a local k3d cluster. 64 closed-loop clients for 90 seconds, once with KEDA scaling workers on the stream's consumer lag and once with a single fixed worker."
      >
        <div className="mt-8 overflow-x-auto">
          <table className="w-full min-w-[40rem] border-t border-ink text-left text-sm">
            <thead>
              <tr className="text-xs text-faint">
                <th scope="col" className={th}>Run</th>
                <th scope="col" className={th}>Jobs</th>
                <th scope="col" className={th}>Jobs/s</th>
                <th scope="col" className={th}>p50</th>
                <th scope="col" className={th}>p99</th>
                <th scope="col" className={th}>Lost</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-rule border-t border-rule">
              {[
                ["KEDA, 1 to 3 workers", r.spike.autoscaled],
                ["One fixed worker", r.spike.oneWorker],
              ].map(([label, l]) => {
                const run = l as typeof r.spike.autoscaled;
                return (
                  <tr key={label as string}>
                    <th scope="row" className="py-3 pr-4 font-semibold">{label as string}</th>
                    <td className={td}>{run.report.completed.toLocaleString()}</td>
                    <td className={td}>{run.report.completed_per_second.toFixed(1)}</td>
                    <td className={td}>{fmtMs(run.report.p50_ms)}</td>
                    <td className={td}>{fmtMs(run.report.p99_ms)}</td>
                    <td className={td}>{run.report.lost}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
        <Source file="2026-11-22-kubernetes.md">
          Measured {reportDate(r.spike.autoscaled.date)}. Replicas were sampled every 2 seconds during the run.
        </Source>
      </Section>

      {r.failureLab.length > 0 && <FailureLab runs={r.failureLab} />}

      <p className="mt-24 max-w-2xl leading-relaxed text-muted">
        Drift detection results are on the <Link href="/drift#experiment" className={link}>drift page</Link>.
      </p>
    </>
  );
}

const FAULT_NAMES: Record<string, string> = {
  none: "No fault",
  "worker-kill": "Worker killed, restarted at heal",
  "nats-restart": "NATS restarted",
  "postgres-pause": "Postgres paused",
  "objectstore-pause": "Object store paused",
};

function FailureLab({ runs }: { runs: FailureRun[] }) {
  const first = runs[0];
  const faults = [...new Set(runs.map((x) => x.fault))];
  return (
    <Section
      id="failure-lab"
      title="Under faults: the bus against a direct call"
      intro={
        <>
          <Code>spotterchaos</Code> ran {first.config.concurrency} closed-loop clients for {first.config.duration_s} s in each mode,
          injected one fault at {first.config.fault_at_s} s, and healed it at {first.config.heal_at_s} s. Asynchronous: submit, then
          wait for the completion event on the bus. Synchronous: the gateway calls the worker over gRPC and answers in the same
          call. The worker ran without a shadow model, so both modes did the same work per image.
        </>
      }
    >
      <div className="mt-8 overflow-x-auto">
        <table className="w-full min-w-[56rem] border-t border-ink text-left text-sm">
          <thead>
            <tr className="text-xs text-faint">
              <th scope="col" className={th}>Fault</th>
              <th scope="col" className={th}>Mode</th>
              <th scope="col" className={th}>Requests</th>
              <th scope="col" className={th}>Per second before the fault</th>
              <th scope="col" className={th}>Rejected</th>
              <th scope="col" className={th}>Lost</th>
              <th scope="col" className={th}>Duplicates</th>
              <th scope="col" className={th}>p99</th>
              <th scope="col" className={th}>Recovery</th>
              <th scope="col" className="py-3 font-medium">Completions per second, each scaled to its own peak (fault shaded, rejections in red)</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-rule border-t border-rule">
            {faults.flatMap((fault) =>
              runs
                .filter((x) => x.fault === fault)
                .map((x, i) => (
                  <tr key={x.file} data-run={`${x.mode}-${x.fault}`}>
                    {i === 0 && (
                      <th scope="rowgroup" rowSpan={runs.filter((y) => y.fault === fault).length} className="py-3 pr-4 align-top font-semibold">
                        {FAULT_NAMES[fault] ?? fault}
                      </th>
                    )}
                    <td className="py-3 pr-4">{x.mode === "async" ? "Bus" : "Direct call"}</td>
                    <td className={td}>{x.report.requests.toLocaleString()}</td>
                    <td className={td}>{x.report.baseline_per_second.toFixed(0)}</td>
                    <td className={`${td} ${x.report.rejected ? "text-fail" : ""}`}>{x.report.rejected.toLocaleString()}</td>
                    <td className={`${td} ${x.report.lost ? "font-semibold text-fail" : ""}`}>{x.report.lost}</td>
                    <td className={`${td} ${x.report.duplicates ? "font-semibold text-fail" : ""}`}>{x.report.duplicates}</td>
                    <td className={td}>{fmtMs(x.report.p99_ms)}</td>
                    <td className={td}>
                      {x.fault === "none" || x.fault === "nats-restart"
                        ? ""
                        : x.report.recovery_seconds === null
                          ? "did not recover"
                          : `${x.report.recovery_seconds} s`}
                    </td>
                    <td className="py-2">
                      <FaultTimeline run={x} max={Math.max(...x.report.completed_per_second)} />
                    </td>
                  </tr>
                )),
            )}
          </tbody>
        </table>
      </div>
      <Source file={`${reportDate(first.date)}-failure-lab.md`}>
        Measured {reportDate(first.date)}. Rejected: the client got an error and could retry. Lost: accepted but never completed.
        Duplicates: more than one stored completion for a job. Recovery: from heal until a second reached 80% of the
        throughput before the fault (not shown for the NATS restart, which has no heal step). One JSON file per run sits next to the write-up.
      </Source>
    </Section>
  );
}
