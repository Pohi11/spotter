import type { NextRequest } from "next/server";

import { GATEWAY_URL } from "@/lib/gateway";
import { allowedPath } from "@/lib/proxy";

// Server-only: the browser reaches the gateway only through here (spec: dashboard).

const notFound = () => Response.json({ message: "not found" }, { status: 404 });

async function forward(request: NextRequest, ctx: { params: Promise<{ path: string[] }> }) {
  const { path } = await ctx.params;
  const target = allowedPath(request.method, path);
  if (target === null) return notFound();

  const headers: Record<string, string> = { accept: request.headers.get("accept") ?? "*/*" };
  let body: string | undefined;
  if (request.method === "POST") {
    headers["content-type"] = "application/json";
    body = await request.text();
  }
  let upstream: Response;
  try {
    upstream = await fetch(new URL(target + request.nextUrl.search, `${GATEWAY_URL}/`), {
      method: request.method,
      headers,
      body,
      cache: "no-store",
      signal: request.signal, // a closed tab stops the upstream event stream too
    });
  } catch {
    return Response.json({ message: "The platform can't be reached from the dashboard." }, { status: 502 });
  }
  // The body passes through as a stream, so server-sent events arrive as they are sent.
  return new Response(upstream.body, {
    status: upstream.status,
    headers: {
      "content-type": upstream.headers.get("content-type") ?? "application/json",
      "cache-control": "no-cache",
    },
  });
}

export const dynamic = "force-dynamic";
export const GET = forward;
export const POST = forward;
export const PUT = notFound;
export const PATCH = notFound;
export const DELETE = notFound;
