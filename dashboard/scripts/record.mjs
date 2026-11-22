// Records real responses from a running platform:
//   node scripts/record.mjs example   → content/worked-example.json (one job, its events,
//                                        and its trace's span timings from Jaeger)
//   node scripts/record.mjs fixtures  → e2e/fixtures/gateway.json (what the stub gateway
//                                        serves to the browser tests)
// Needs GATEWAY_URL (default http://localhost:8080) and, for the example, JAEGER_URL
// (default http://localhost:16686).
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const GATEWAY = process.env.GATEWAY_URL ?? "http://localhost:8080";
const JAEGER = process.env.JAEGER_URL ?? "http://localhost:16686";
const MODEL = process.env.MODEL_NAME ?? "eurosat-classifier";
const manifest = JSON.parse(readFileSync(join(root, "public", "samples", "manifest.json"), "utf-8"));

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function json(path, init) {
  const res = await fetch(`${GATEWAY}${path}`, init);
  const body = await res.json();
  if (!res.ok) throw new Error(`${path}: ${res.status} ${JSON.stringify(body)}`);
  return body;
}

async function submit(file) {
  const image = readFileSync(join(root, "public", "samples", file)).toString("base64");
  const { jobId } = await json(`/v1/models/${MODEL}/inferences`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ image, contentType: "image/jpeg" }),
  });
  for (let i = 0; i < 100; i++) {
    const { job } = await json(`/v1/inferences/${jobId}`);
    if (job.status !== "JOB_STATUS_PENDING") return job;
    await sleep(50);
  }
  throw new Error(`job ${jobId} still pending`);
}

/** Reads the event feed for a few seconds and returns what arrived. */
async function events(since, seconds) {
  const ctl = new AbortController();
  const res = await fetch(`${GATEWAY}/v1/events?since=${since}`, { signal: ctl.signal });
  const out = [];
  const reader = res.body.getReader();
  const timer = setTimeout(() => ctl.abort(), seconds * 1000);
  let buf = "";
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      buf += new TextDecoder().decode(value);
      let i;
      while ((i = buf.indexOf("\n\n")) >= 0) {
        const block = buf.slice(0, i);
        buf = buf.slice(i + 2);
        const data = block.split("\n").find((l) => l.startsWith("data: "));
        if (data) out.push(JSON.parse(data.slice(6)));
      }
    }
  } catch (e) {
    if (e.name !== "AbortError") throw e;
  } finally {
    clearTimeout(timer);
  }
  return out;
}

async function trace(jobId) {
  const now = new Date();
  const q = new URLSearchParams({
    "query.service_name": "gateway",
    "query.attributes": JSON.stringify({ "spotter.job_id": jobId }),
    "query.start_time_min": new Date(now - 10 * 60_000).toISOString(),
    "query.start_time_max": now.toISOString(),
    "query.search_depth": "5",
  });
  for (let i = 0; i < 20; i++) {
    const res = await fetch(`${JAEGER}/api/v3/traces?${q}`);
    if (res.ok) {
      const spans = [];
      for (const rs of (await res.json()).result.resourceSpans) {
        const service = rs.resource.attributes.find((a) => a.key === "service.name").value.stringValue;
        for (const ss of rs.scopeSpans)
          for (const s of ss.spans)
            spans.push({ service, name: s.name, start: BigInt(s.startTimeUnixNano), end: BigInt(s.endTimeUnixNano) });
      }
      if (spans.some((s) => s.service === "inference-worker")) {
        const t0 = spans.reduce((m, s) => (s.start < m ? s.start : m), spans[0].start);
        return spans
          .sort((a, b) => (a.start < b.start ? -1 : 1))
          .map((s) => ({
            service: s.service,
            name: s.name,
            start_ms: Number(s.start - t0) / 1e6,
            duration_ms: Number(s.end - s.start) / 1e6,
          }));
      }
    }
    await sleep(500); // spans are exported in batches
  }
  throw new Error(`no complete trace for job ${jobId}`);
}

async function example() {
  const forest = manifest.samples.find((s) => s.true_class === "Forest");
  const clean = forest.variants.find((v) => v.degradation === "none");
  // Warm up the worker and connections so the example shows steady-state timings; the
  // warm-up jobs' results are the home page's contact sheet.
  const contact = [];
  for (const s of manifest.samples) {
    const j = await submit(s.variants[0].file);
    contact.push({ id: s.id, file: s.variants[0].file, true_class: s.true_class, label: j.prediction?.label ?? null,
      confidence: j.prediction?.confidence ?? null, version: j.prediction?.modelVersion ?? null });
  }
  const job = await submit(clean.file);
  const spans = await trace(job.jobId);
  const feed = (await events("2m", 2)).filter((e) => e.payload?.jobId === job.jobId);
  const out = {
    about: "One real job, recorded by scripts/record.mjs from a running platform and its Jaeger traces.",
    recorded_at: new Date().toISOString(),
    sample: { id: forest.id, file: clean.file, true_class: forest.true_class, source: forest.source },
    job,
    events: feed,
    spans,
    contact,
  };
  writeFileSync(join(root, "content", "worked-example.json"), JSON.stringify(out, null, 1) + "\n");
  console.log(`recorded job ${job.jobId}: ${job.prediction.label} ${job.prediction.confidence}, ${spans.length} spans`);
}

async function fixtures() {
  const model = await json(`/v1/models/${MODEL}`);
  const { versions } = await json(`/v1/models/${MODEL}/versions`);
  const perVersion = {};
  for (const v of versions) {
    perVersion[v.id] = {
      version: await json(`/v1/versions/${v.id}`),
      history: await json(`/v1/versions/${v.id}/history`),
      gates: await json(`/v1/versions/${v.id}/gates`),
    };
  }
  const forest = manifest.samples.find((s) => s.true_class === "Forest");
  const job = await submit(forest.variants[0].file);
  // Only this model's events: tests elsewhere publish to the same streams under other names.
  const feed = (await events("30m", 4)).filter((e) => e.subject.endsWith(`.${MODEL}`) || e.stream === "REGISTRY");
  const keep = [];
  const seen = {};
  for (const e of feed.reverse()) {
    const n = (seen[e.type] = (seen[e.type] ?? 0) + 1);
    if (n <= (e.type === "spotter.v1.DriftEvaluated" ? 40 : 6)) keep.push(e);
  }
  const out = { recorded_at: new Date().toISOString(), model, versions: { versions }, perVersion, job: { job }, events: keep.reverse() };
  mkdirSync(join(root, "e2e", "fixtures"), { recursive: true });
  writeFileSync(join(root, "e2e", "fixtures", "gateway.json"), JSON.stringify(out, null, 1) + "\n");
  console.log(`recorded ${versions.length} versions and ${keep.length} events`);
}

const mode = process.argv[2];
if (mode === "example") await example();
else if (mode === "fixtures") await fixtures();
else {
  console.error("usage: node scripts/record.mjs example|fixtures");
  process.exit(2);
}
