// Which browser requests the dashboard forwards to the gateway (spec: dashboard "The browser
// reaches the gateway only through an allowlisted proxy"). Everything else is answered 404
// without contacting the gateway, so the browser reaches only what the pages need.

const ID = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}";
const NAME = "[a-z0-9][a-z0-9-]{0,62}";

const RULES: ReadonlyArray<{ method: string; pattern: RegExp }> = [
  { method: "GET", pattern: new RegExp(`^v1/models/${NAME}$`) },
  { method: "GET", pattern: new RegExp(`^v1/models/${NAME}/versions$`) },
  { method: "GET", pattern: new RegExp(`^v1/versions/${ID}$`) },
  { method: "GET", pattern: new RegExp(`^v1/versions/${ID}/(history|gates)$`) },
  { method: "POST", pattern: new RegExp(`^v1/versions/${ID}:(approve|promote)$`) },
  { method: "POST", pattern: new RegExp(`^v1/models/${NAME}:rollback$`) },
  { method: "POST", pattern: new RegExp(`^v1/models/${NAME}/inferences$`) },
  { method: "GET", pattern: new RegExp(`^v1/inferences/${ID}$`) },
  { method: "GET", pattern: /^v1\/events$/ },
];

/** The gateway path to call, or null when the request must not be forwarded. */
export function allowedPath(method: string, segments: readonly string[]): string | null {
  const path = segments.map(decodeURIComponent).join("/");
  if (path.includes("..")) return null;
  return RULES.some((rule) => rule.method === method && rule.pattern.test(path)) ? path : null;
}
