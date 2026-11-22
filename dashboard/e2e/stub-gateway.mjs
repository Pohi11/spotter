// A stand-in for the gateway in browser tests (design D7 of change phase-10-dashboard). It
// serves JSON recorded from the real gateway by `node scripts/record.mjs fixtures`, keeps
// the registry's state so approve, promote, and rollback behave as the registry does, and
// replays the recorded event stream. Test hooks under /__ change its state.
//
//   node e2e/stub-gateway.mjs [port]   (default 8099)
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const fixtures = JSON.parse(readFileSync(join(dirname(fileURLToPath(import.meta.url)), "fixtures", "gateway.json"), "utf-8"));
const PORT = Number(process.argv[2] ?? process.env.STUB_PORT ?? 8099);

let state;
function reset() {
  state = structuredClone({ versions: fixtures.versions.versions, perVersion: fixtures.perVersion, events: fixtures.events, polls: {} });
}
reset();
const clients = new Set();

const json = (res, status, body) => {
  res.writeHead(status, { "content-type": "application/json" });
  res.end(JSON.stringify(body));
};
const refuse = (res, message) => json(res, 400, { code: 9, message, details: [] });
const now = () => new Date().toISOString();

function version(id) {
  return state.versions.find((v) => v.id === id);
}

function setStage(v, stage, reason) {
  const from = v.stage;
  v.stage = stage;
  v.revision = String(Number(v.revision) + 1);
  v.updatedAt = now();
  state.perVersion[v.id].version.version = v;
  state.perVersion[v.id].history.changes.push({ fromStage: from, toStage: stage, reason, revision: v.revision, changedAt: v.updatedAt });
  emit({ stream: "REGISTRY", subject: "registry.events.model_version_stage_changed", type: "spotter.v1.ModelVersionStageChanged",
    payload: { modelName: fixtures.model.model.name, versionId: v.id, version: v.version, fromStage: from, toStage: stage, reason } });
}

function recomputeGates(id) {
  const g = state.perVersion[id].gates;
  g.allPassed = g.gates.every((x) => x.passed);
}

let seq = 100000;
function emit(ev) {
  const full = { sequence: ++seq, time: now(), ...ev };
  state.events.push(full);
  for (const res of clients) send(res, full);
}
function send(res, ev) {
  res.write(`id: ${ev.stream}:${ev.sequence}\ndata: ${JSON.stringify(ev)}\n\n`);
}

async function body(req) {
  let s = "";
  for await (const chunk of req) s += chunk;
  return s ? JSON.parse(s) : {};
}

const server = createServer(async (req, res) => {
  const url = new URL(req.url, `http://localhost:${PORT}`);
  const p = url.pathname;
  const m = (re) => p.match(re);
  let g;

  // Test hooks.
  if (req.method === "POST" && p === "/__reset") return reset(), json(res, 200, {});
  if (req.method === "POST" && p === "/__emit") return emit(await body(req)), json(res, 200, {});
  if (req.method === "POST" && p === "/__drop") {
    for (const c of clients) c.destroy();
    clients.clear();
    return json(res, 200, {});
  }
  if (req.method === "POST" && p === "/__gate") {
    const { version: n, name, passed, detail } = await body(req);
    const v = state.versions.find((x) => x.version === n);
    const gate = state.perVersion[v.id].gates.gates.find((x) => x.name === name);
    Object.assign(gate, { passed, detail });
    recomputeGates(v.id);
    return json(res, 200, {});
  }

  if (req.method === "GET" && p === "/readyz") return res.end("ok\n");
  if (req.method === "GET" && p === "/v1/events") {
    res.writeHead(200, { "content-type": "text/event-stream", "cache-control": "no-cache" });
    res.write(": connected\n\n");
    const streams = url.searchParams.get("streams")?.split(",");
    for (const ev of state.events) if (!streams || streams.includes(ev.stream)) send(res, ev);
    clients.add(res);
    req.on("close", () => clients.delete(res));
    return;
  }
  if (req.method === "GET" && (g = m(/^\/v1\/models\/([^/:]+)$/))) return json(res, 200, fixtures.model);
  if (req.method === "GET" && (g = m(/^\/v1\/models\/([^/:]+)\/versions$/))) return json(res, 200, { versions: state.versions });
  if (req.method === "GET" && (g = m(/^\/v1\/versions\/([^/:]+)$/))) return json(res, 200, { version: version(g[1]) });
  if (req.method === "GET" && (g = m(/^\/v1\/versions\/([^/:]+)\/history$/))) return json(res, 200, state.perVersion[g[1]].history);
  if (req.method === "GET" && (g = m(/^\/v1\/versions\/([^/:]+)\/gates$/))) return json(res, 200, state.perVersion[g[1]].gates);

  if (req.method === "POST" && (g = m(/^\/v1\/versions\/([^/:]+):approve$/))) {
    const { approver, reason } = await body(req);
    if (!approver || !reason) return json(res, 400, { code: 3, message: "approver and reason are required", details: [] });
    const gate = state.perVersion[g[1]].gates.gates.find((x) => x.name === "approval");
    Object.assign(gate, { passed: true, detail: `1 approval(s) recorded` });
    recomputeGates(g[1]);
    return json(res, 200, { versionId: g[1], approver, approvedAt: now() });
  }
  if (req.method === "POST" && (g = m(/^\/v1\/versions\/([^/:]+):promote$/))) {
    const gates = state.perVersion[g[1]].gates;
    const failed = gates.gates.filter((x) => !x.passed).map((x) => x.name);
    if (failed.length) return refuse(res, `promotion gates not passed: ${failed.join(", ")}`);
    const prod = state.versions.find((v) => v.stage === "MODEL_VERSION_STAGE_PRODUCTION");
    const cand = version(g[1]);
    if (prod) setStage(prod, "MODEL_VERSION_STAGE_RETIRED", `replaced by version ${cand.version}`);
    setStage(cand, "MODEL_VERSION_STAGE_PRODUCTION", "promoted");
    return json(res, 200, { promoted: cand, retired: prod ?? null });
  }
  if (req.method === "POST" && (g = m(/^\/v1\/models\/([^/:]+):rollback$/))) {
    const prod = state.versions.find((v) => v.stage === "MODEL_VERSION_STAGE_PRODUCTION");
    const target = state.versions
      .filter((v) => v.stage === "MODEL_VERSION_STAGE_RETIRED")
      .sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt))[0];
    if (!prod || !target) return refuse(res, "nothing to roll back to");
    const { reason } = await body(req);
    setStage(prod, "MODEL_VERSION_STAGE_ROLLED_BACK", reason);
    setStage(target, "MODEL_VERSION_STAGE_PRODUCTION", `restored by rollback of version ${prod.version}: ${reason}`);
    return json(res, 200, { restored: target, rolledBack: prod });
  }
  if (req.method === "POST" && (g = m(/^\/v1\/models\/([^/:]+)\/inferences$/))) {
    await body(req);
    const jobId = fixtures.job.job.jobId;
    state.polls[jobId] = 0;
    return json(res, 200, { jobId });
  }
  if (req.method === "GET" && (g = m(/^\/v1\/inferences\/([^/:]+)$/))) {
    // Pending on the first poll, then the recorded result, as a real job goes.
    const n = (state.polls[g[1]] = (state.polls[g[1]] ?? 0) + 1);
    const job = structuredClone(fixtures.job.job);
    if (n === 1) Object.assign(job, { status: "JOB_STATUS_PENDING", prediction: null, completedAt: null });
    return json(res, 200, { job });
  }
  json(res, 404, { code: 5, message: `stub: no route for ${req.method} ${p}`, details: [] });
});

server.listen(PORT, () => console.log(`stub gateway on http://localhost:${PORT}`));
