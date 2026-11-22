import { NextRequest } from "next/server";
import { afterEach, expect, test, vi } from "vitest";

import { GET, POST } from "@/app/api/[...path]/route";

import { allowedPath } from "./proxy";
import { spec } from "./spec";

const REQ = spec("dashboard", "The browser reaches the gateway only through an allowlisted proxy");
const ID = "01a204cd-c722-72b8-ae4c-8ca118f6b184";
const seg = (p: string) => p.split("/");

test(`forwards the routes the pages need ${REQ}`, () => {
  const allowed: [string, string][] = [
    ["GET", "v1/models/eurosat-classifier"],
    ["GET", "v1/models/eurosat-classifier/versions"],
    ["GET", `v1/versions/${ID}`],
    ["GET", `v1/versions/${ID}/history`],
    ["GET", `v1/versions/${ID}/gates`],
    ["POST", `v1/versions/${ID}:approve`],
    ["POST", `v1/versions/${ID}:promote`],
    ["POST", "v1/models/eurosat-classifier:rollback"],
    ["POST", "v1/models/eurosat-classifier/inferences"],
    ["GET", `v1/inferences/${ID}`],
    ["GET", "v1/events"],
  ];
  for (const [method, path] of allowed) expect(allowedPath(method, seg(path)), `${method} ${path}`).toBe(path);
});

test(`refuses everything else ${REQ}`, () => {
  const refused: [string, string][] = [
    ["POST", "v1/models"],
    ["GET", "openapi.json"],
    ["POST", `v1/versions/${ID}:transition`],
    ["POST", "v1/models/eurosat-classifier/versions"],
    ["GET", "v1/models/eurosat-classifier:rollback"],
    ["GET", "metrics"],
    ["GET", "v1/models/../openapi.json"],
    ["GET", `v1/versions/not-an-id/gates`],
  ];
  for (const [method, path] of refused) expect(allowedPath(method, seg(path)), `${method} ${path}`).toBeNull();
});

afterEach(() => vi.unstubAllGlobals());

const ctx = (path: string) => ({ params: Promise.resolve({ path: seg(path) }) });

test(`answers 404 without calling the gateway ${REQ}`, async () => {
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  const res = await POST(new NextRequest("http://dash/api/v1/models", { method: "POST", body: "{}" }), ctx("v1/models"));
  expect(res.status).toBe(404);
  const res2 = await GET(new NextRequest("http://dash/api/openapi.json"), ctx("openapi.json"));
  expect(res2.status).toBe(404);
  expect(fetch).not.toHaveBeenCalled();
});

test(`passes the gateway's status and body through ${REQ}`, async () => {
  const fetch = vi.fn(async () => new Response('{"versions":[]}', { status: 200, headers: { "content-type": "application/json" } }));
  vi.stubGlobal("fetch", fetch);
  const res = await GET(new NextRequest("http://dash/api/v1/models/eurosat-classifier/versions"), ctx("v1/models/eurosat-classifier/versions"));
  expect(res.status).toBe(200);
  expect(await res.text()).toBe('{"versions":[]}');
  const called = String((fetch.mock.calls[0] as unknown[])[0]);
  expect(called).toMatch(/\/v1\/models\/eurosat-classifier\/versions$/);
});

test(`says so when the gateway is unreachable ${REQ}`, async () => {
  vi.stubGlobal("fetch", vi.fn(async () => Promise.reject(new TypeError("fetch failed"))));
  const res = await GET(new NextRequest("http://dash/api/v1/events"), ctx("v1/events"));
  expect(res.status).toBe(502);
  expect((await res.json()).message).toMatch(/can't be reached/);
});
