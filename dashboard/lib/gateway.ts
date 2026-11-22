// Server-side access to the gateway, for server components and the proxy route only. The
// gateway's address never reaches the browser.

export const GATEWAY_URL = process.env.GATEWAY_URL ?? "http://localhost:8080";
export const MODEL_NAME = process.env.MODEL_NAME ?? "eurosat-classifier";

export class GatewayError extends Error {
  constructor(
    readonly status: number | null,
    readonly path: string,
  ) {
    super(status === null ? `gateway unreachable for ${path}` : `gateway answered ${status} for ${path}`);
  }
}

export async function gatewayJson<T>(path: string): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`${GATEWAY_URL}${path}`, { cache: "no-store" });
  } catch {
    throw new GatewayError(null, path);
  }
  if (!response.ok) throw new GatewayError(response.status, path);
  return (await response.json()) as T;
}
