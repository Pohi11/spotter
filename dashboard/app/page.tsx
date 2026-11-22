import { readFileSync } from "node:fs";
import { join } from "node:path";

import Link from "next/link";
import type { ReactNode } from "react";

import { ClassifyPanel } from "@/components/classify-panel";
import { ContactSheet } from "@/components/contact-sheet";
import { Code, Dot, Figure, Section, fmtMs, link, pct } from "@/components/ui";
import { Waterfall } from "@/components/waterfall";
import worked from "@/content/worked-example.json";
import { className } from "@/lib/classes";
import type { Tone } from "@/lib/events";
import { MODEL_NAME } from "@/lib/gateway";
import { loadResults } from "@/lib/results";
import type { Sample } from "@/lib/types";

// The home page (spec: dashboard). The contact sheet, the worked example, and the figures
// come from recorded runs and dated reports; the prose is written by hand and has to
// match the code.

function samples(): Sample[] {
  const manifest = JSON.parse(readFileSync(join(process.cwd(), "public", "samples", "manifest.json"), "utf-8"));
  return manifest.samples as Sample[];
}

const SKILLS_SHORT: { skill: string; what: string }[] = [
  { skill: "Go", what: "the registry, gateway, and drift monitor, as hexagonal services" },
  { skill: "Python and PyTorch", what: "training, ONNX export, int8 quantization, and the inference worker" },
  { skill: "NATS JetStream", what: "every job and state change travels as a protobuf event" },
  { skill: "gRPC and REST", what: "one protobuf file defines both APIs and the OpenAPI document" },
  { skill: "PostgreSQL", what: "the registry's invariants live in the schema, not only in code" },
  { skill: "Kubernetes", what: "one Helm chart, KEDA scaling on queue depth, tested on kind in CI" },
];

export default function Home() {
  const results = loadResults();
  const sampleList = samples();
  return (
    <>
      <section className="grid items-start gap-10 lg:grid-cols-[minmax(0,7fr)_minmax(0,5fr)] lg:gap-12">
        <div className="max-w-2xl lg:pt-2">
          <h1 className="text-4xl leading-[1.08] font-semibold sm:text-5xl">
            Spotter serves a satellite-image classifier and watches it for drift.
          </h1>
          <p className="mt-6 text-lg leading-relaxed text-muted">
            Spotter is a small, cloud-neutral platform for running computer-vision models in
            production. It keeps a registry of model versions, serves the current one behind
            REST and gRPC through an event bus, and compares live traffic with what the model
            was trained on.
          </p>
          <p className="mt-4 text-lg leading-relaxed text-muted">
            When the inputs shift, Spotter raises a drift event. A new version runs in shadow
            next to production, and it reaches production only when code-checked gates pass and
            a person approves. One call rolls it back.
          </p>
          <p className="mt-4 text-lg leading-relaxed">
            The model labels 64-pixel Sentinel-2 tiles from the public EuroSAT dataset with one
            of ten land uses. The platform around it uses:
          </p>
          <ul className="mt-4 space-y-2">
            {SKILLS_SHORT.map((f) => (
              <li key={f.skill} className="flex gap-3 leading-relaxed">
                <span aria-hidden className="mt-2.5 h-1.5 w-1.5 shrink-0 rounded-full bg-pass" />
                <span>
                  <span className="font-medium">{f.skill}:</span> <span className="text-muted">{f.what}</span>
                </span>
              </li>
            ))}
          </ul>
          <div className="mt-8 flex flex-wrap items-center gap-x-5 gap-y-3">
            <a href="#try" className="rounded-full bg-ink px-5 py-2.5 font-medium text-white hover:brightness-125">
              Classify a tile
            </a>
            <a href="#example" className={link}>
              Follow one tile
            </a>
            <Link href="/architecture" className={link}>
              See the architecture
            </Link>
          </div>
        </div>
        <ContactSheet tiles={worked.contact} highlight={worked.sample.id} recordedAt={worked.recorded_at} />
      </section>

      <WorkedExample />

      <section id="try" aria-labelledby="try-heading" className="mt-28 scroll-mt-6">
        <h2 id="try-heading" className="text-3xl font-semibold">
          Classify a tile
        </h2>
        <p className="mt-3 mb-8 max-w-2xl leading-relaxed text-muted">
          Pick one of the ten holdout tiles, optionally degrade it, and send it through the
          platform. The tile goes to the gateway, onto the bus, and through whichever version is
          in production right now. The <Link href="/live" className={link}>live events</Link> page
          shows the same job arriving on the bus.
        </p>
        <ClassifyPanel samples={sampleList} model={MODEL_NAME} />
      </section>

      <HowItMoves />
      <Guarantees />
      <Measured results={results} />
      <Method />
      <Skills />
    </>
  );
}

function WorkedExample() {
  const job = worked.job;
  const p = job.prediction!;
  const total = Math.max(...worked.spans.map((s) => s.start_ms + s.duration_ms));
  const store = worked.spans.find((s) => s.name === "store image");
  const classify = worked.spans.find((s) => s.name === "classify");
  const facts: { tone: Tone; label: string; value: ReactNode }[] = [
    { tone: "code", label: "Job", value: <span className="font-mono text-[0.9em]">{job.jobId}</span> },
    { tone: "model", label: "Result", value: `${className(p.label)}, ${pct(p.confidence)} confident` },
    { tone: "pass", label: "Served by", value: `version ${p.modelVersion}, the production version at the time` },
    { tone: "bus", label: "Events", value: `${worked.events.length} on the bus: ${worked.events.map((e) => e.type.replace("spotter.v1.", "")).join(" and ")}` },
  ];
  return (
    <Section
      id="example"
      title="One tile, start to finish"
      intro={
        <>
          The outlined forest tile, submitted to the running platform and recorded with its
          OpenTelemetry trace. Every row is a span: one trace follows the tile from the gateway
          (Go) across NATS to the inference worker (Python) and back onto the bus.
        </>
      }
    >
      <div className="mt-10 grid gap-x-10 gap-y-12">
        <div className="flex flex-col gap-6 sm:flex-row sm:items-start">
          {/* eslint-disable-next-line @next/next/no-img-element -- the recorded tile */}
          <img
            src={`/samples/${worked.sample.file}`}
            alt="The forest tile from the recorded run"
            width={64}
            height={64}
            className="h-36 w-36 shrink-0 rounded-lg [image-rendering:pixelated]"
          />
          <dl className="max-w-xl flex-1 space-y-2.5 text-sm">
            {facts.map((f) => (
              <div key={f.label} className="grid grid-cols-[1rem_5.5rem_minmax(0,1fr)] items-baseline">
                <Dot tone={f.tone} />
                <dt className="text-muted">{f.label}</dt>
                <dd className="min-w-0 break-words">{f.value}</dd>
              </div>
            ))}
          </dl>
        </div>
        <div>
          <Waterfall spans={worked.spans} />
          <p className="mt-6 max-w-2xl leading-relaxed text-muted">
            The whole path took {fmtMs(total)}. The model itself ran for {fmtMs(classify?.duration_ms ?? 0)}.
            Writing the 3 KB tile to the object store took {fmtMs(store?.duration_ms ?? 0)}, most of
            the submit call, and its cause is still open (the{" "}
            <Link href="/results#load" className={link}>results</Link> page has the measurements).
            Recorded {new Date(worked.recorded_at).toLocaleString("en-US", { dateStyle: "long", timeStyle: "short", timeZone: "UTC" })} UTC.
          </p>
        </div>
      </div>
    </Section>
  );
}

const STEPS: { title: string; who: Tone; whoLabel: string; text: ReactNode }[] = [
  {
    title: "Submit",
    who: "code",
    whoLabel: "gateway, Go",
    text: (
      <>
        The gateway checks the model exists, stores the tile in the S3-compatible object store,
        records the job as pending in a JetStream key-value bucket, and publishes{" "}
        <Code>inference.requested</Code>. The caller gets a job id at once.
      </>
    ),
  },
  {
    title: "Queue",
    who: "bus",
    whoLabel: "NATS JetStream",
    text: "The job waits on a durable stream. Workers pull from one shared consumer, so each job goes to one worker, and KEDA adds workers when the backlog grows.",
  },
  {
    title: "Classify",
    who: "model",
    whoLabel: "worker, Python",
    text: "The worker runs the production version (ONNX Runtime, int8). If a shadow version is loaded, it runs too, and only production's answer counts.",
  },
  {
    title: "Record once",
    who: "model",
    whoLabel: "worker, Python",
    text: (
      <>
        The worker writes the result only if the job is still pending (compare-and-set), publishes{" "}
        <Code>inference.completed</Code> with a de-duplicating message id, and only then
        acknowledges. A crash at any point means a retry, never a second result.
      </>
    ),
  },
  {
    title: "Watch",
    who: "code",
    whoLabel: "monitor, Go",
    text: "The drift monitor reads every completion, keeps the last 200 per version, and every 50 compares them with the version's training data. A shift publishes a drift event.",
  },
  {
    title: "Decide",
    who: "human",
    whoLabel: "registry, Go, and a person",
    text: "A candidate version is promoted only when the accuracy, shadow-agreement, and latency gates pass in code and a person has approved it. Rollback restores the previous version in one transaction.",
  },
];

function HowItMoves() {
  return (
    <Section id="how" title="How a tile moves" intro="Each step is owned by one service. They talk only through the gateway's APIs and the event bus.">
      <ol className="mt-10 max-w-3xl">
        {STEPS.map((step, i) => (
          <li key={step.title} className="relative grid grid-cols-[2.5rem_minmax(0,1fr)] pb-9 last:pb-0">
            {i < STEPS.length - 1 && <span aria-hidden className="absolute top-8 bottom-1 left-[0.9rem] w-px bg-rule" />}
            <span className="flex h-7 w-7 items-center justify-center rounded-full bg-surface text-sm font-semibold ring-1 ring-rule">{i + 1}</span>
            <div>
              <h3 className="flex flex-wrap items-baseline gap-x-3 text-xl font-semibold">
                {step.title}
                <span className="inline-flex items-center gap-1.5 text-sm font-normal text-muted">
                  <Dot tone={step.who} />
                  {step.whoLabel}
                </span>
              </h3>
              <p className="mt-1.5 leading-relaxed text-muted">{step.text}</p>
            </div>
          </li>
        ))}
      </ol>
    </Section>
  );
}

const GUARANTEES: ReactNode[] = [
  "A job ends with exactly one result and one completion event, even when the worker stops before recording it or before acknowledging. Tests stop it at both points and count the results.",
  "A stage change and its event commit in the same Postgres transaction (a transactional outbox), so the bus never misses a change and never announces one that rolled back.",
  "Postgres refuses a second production version of a model with a partial unique index. When two transitions race, exactly one wins.",
  "No model output can promote, approve, or deploy anything. Promotion gates are plain Go with unit tests, and promotion needs a recorded human approval.",
  "The domain and use-case packages can't import Postgres, NATS, S3, gRPC, or HTTP. A test reads the import graph and fails the build if they do.",
  <>
    Every requirement in the specs has at least one passing test that names it, in Go, Python,
    and TypeScript. CI fails otherwise. <Link href="/build" className={link}>How it was built</Link> links each one to its tests.
  </>,
];

function Guarantees() {
  return (
    <Section id="guarantees" title="What the tests hold it to">
      <ul className="mt-8 grid max-w-4xl gap-x-10 gap-y-5 leading-relaxed text-muted md:grid-cols-2">
        {GUARANTEES.map((g, i) => (
          <li key={i} className="flex gap-3">
            <span aria-hidden className="mt-2.5 h-1.5 w-1.5 shrink-0 rounded-full bg-pass" />
            <span>{g}</span>
          </li>
        ))}
      </ul>
    </Section>
  );
}

function Measured({ results }: { results: ReturnType<typeof loadResults> }) {
  const [fp32, , int8] = results.variants.variants;
  const b1 = (v: typeof fp32) => v.latency.find((l) => l.batch === 1)!.p50_ms;
  const e2e = results.load.find((l) => l.file.includes("closed-loop-onnx-int8.json"))!;
  const { autoscaled, oneWorker } = results.spike;
  return (
    <Section
      id="measured"
      title="Measured, not estimated"
      intro={
        <>
          Every number here comes from a dated report in the repository, produced by the code
          that measured it. The <Link href="/results" className={link}>results</Link> page has the
          full tables and how each was measured.
        </>
      }
    >
      <dl className="mt-10 grid grid-cols-1 gap-x-8 gap-y-8 sm:grid-cols-2 lg:grid-cols-3">
        <Figure label="Holdout accuracy, int8 model" value={pct(int8.accuracy, 2)} note={`${results.variants.holdout_tiles.toLocaleString()} tiles never seen in training`} />
        <Figure label="int8 against PyTorch fp32, batch of 1" value={`${(b1(fp32) / b1(int8)).toFixed(1)}× faster`} note={`p50 ${fmtMs(b1(int8))} against ${fmtMs(b1(fp32))}, ${(fp32.size_bytes / int8.size_bytes).toFixed(1)}× smaller`} />
        <Figure label="End to end, one worker" value={`${e2e.report.completed_per_second.toFixed(0)} jobs/s`} note={`p50 ${fmtMs(e2e.report.p50_ms)}, p99 ${fmtMs(e2e.report.p99_ms)}, ${e2e.report.lost} lost`} />
        <Figure label="False drift alarms on clean traffic" value={`${results.drift.false_alarms} of ${results.drift.clean_events.toLocaleString()}`} note="events, with PSI thresholds of 0.2 to raise and 0.1 to clear" />
        <Figure label="Rollback to the worker serving the old version" value={fmtMs(results.promotion.rollback_to_serving_ms)} note={`${results.promotion.rollback_under_load.lost} jobs lost when done under load`} />
        <Figure label="Load spike on Kubernetes, KEDA from 1 to 3 workers" value={`${autoscaled.report.completed_per_second.toFixed(0)} jobs/s`} note={`against ${oneWorker.report.completed_per_second.toFixed(0)} jobs/s with one fixed worker`} />
      </dl>
    </Section>
  );
}

function Method() {
  const steps = [
    "Each piece of work starts as an OpenSpec change: requirements written as when-then scenarios, a design that weighs the options, and a task list.",
    "A choice that outlives one change gets a numbered decision record with the options considered. A committed record is never rewritten; a new one supersedes it.",
    "Code lands in small commits. Lint, type checks, and tests pass before each one, and Go tests run with the race detector against real Postgres and NATS.",
    "Each test names the requirement it proves, and CI fails when a requirement has no passing test. Numbers are reported only after they are measured, with how.",
  ];
  return (
    <Section id="method" title="How it was built" intro="Spotter was built one change at a time, each going through the same four steps.">
      <ol className="mt-8 max-w-2xl list-decimal space-y-3 pl-6 leading-relaxed marker:font-semibold marker:text-pass">
        {steps.map((s) => (
          <li key={s} className="pl-2">
            {s}
          </li>
        ))}
      </ol>
      <p className="mt-8 max-w-2xl leading-relaxed text-muted">
        <Link href="/build" className={link}>How it was built</Link> lists every change with its
        requirements, the tests behind them, and their latest results, and every decision record.
      </p>
    </Section>
  );
}

const STACK: { skill: string; here: ReactNode }[] = [
  { skill: "Go beyond coursework", here: "Three services (registry, gateway, monitor), a CLI, a load generator, and the spec-coverage tool. Hexagonal layout with ports and adapters, contexts and graceful shutdown everywhere, tests run with -race." },
  { skill: "Python and PyTorch", here: "A CNN trained on EuroSAT, exported to ONNX and quantized to int8, served by an asyncio worker that verifies each artifact's SHA-256 before loading it." },
  { skill: "Event-driven microservices", here: "NATS JetStream streams and a key-value bucket defined in code, at-least-once delivery with idempotent consumers, a transactional outbox, and dead letters." },
  { skill: "REST and gRPC", here: "grpc-go and grpc-gateway from one protobuf file, with buf lint and buf breaking in CI. The dashboard follows the bus live over server-sent events." },
  { skill: "Relational and non-relational data", here: "PostgreSQL for the registry (migrations, partial unique index, optimistic concurrency); JetStream key-value for job state; an S3-compatible object store for artifacts and tiles." },
  { skill: "Kubernetes and Everything as Code", here: "One Helm chart for the whole platform, KEDA scaling workers on consumer lag, Tilt and k3d for local work, and a kind cluster in CI that installs the chart and runs a smoke test." },
  { skill: "Observability", here: "OpenTelemetry traces across HTTP, gRPC, and NATS headers (even through the outbox), Prometheus metrics, and a drift monitor based on PSI and the KS test." },
  { skill: "Cloud-neutral design", here: "No cloud SDK outside an adapter, an S3 port tested against SeaweedFS, and everything runs on a laptop." },
  { skill: "TypeScript and React", here: "This dashboard, in Next.js and React with Tailwind CSS, tested with Vitest and Playwright." },
];

function Skills() {
  return (
    <Section id="skills" title="Skills across the stack" intro="Each row pairs a skill with the parts of Spotter that use it.">
      <table className="mt-10 w-full max-w-4xl border-t border-ink text-left">
        <thead className="sr-only">
          <tr>
            <th scope="col">Skill</th>
            <th scope="col">Where Spotter uses it</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-rule">
          {STACK.map((row) => (
            <tr key={row.skill} className="flex flex-col gap-1 py-4 sm:table-row">
              <th scope="row" className="pr-8 align-top font-semibold sm:w-64 sm:py-4">
                {row.skill}
              </th>
              <td className="align-top leading-relaxed text-muted sm:py-4">{row.here}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </Section>
  );
}
