// Copies the dated benchmark reports from ../benchmarks into content/benchmarks, so the
// results page reads them at build time (spec: dashboard "Results come from dated
// reports"). The copies are not committed; benchmarks/ stays the one source.
import { copyFileSync, existsSync, mkdirSync, readdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const from = process.env.BENCHMARKS_DIR ?? join(root, "..", "benchmarks");
const to = join(root, "content", "benchmarks");
if (!existsSync(from)) {
  if (existsSync(to)) process.exit(0); // a Docker build context brings its own copy
  console.error(`sync-content: no benchmarks at ${from}`);
  process.exit(1);
}
mkdirSync(to, { recursive: true });
const files = readdirSync(from).filter((f) => f.endsWith(".json"));
for (const f of files) copyFileSync(join(from, f), join(to, f));
console.log(`sync-content: ${files.length} reports copied to content/benchmarks`);
