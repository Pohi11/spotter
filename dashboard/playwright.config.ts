import { defineConfig, devices } from "@playwright/test";

// Browser tests (spec: dashboard; design D7): the dashboard in front of a stub gateway that
// serves responses recorded from the real gateway, so they run without the platform.
const STUB_PORT = Number(process.env.SPOTTER_E2E_STUB_PORT ?? 8099);
const WEB_PORT = Number(process.env.SPOTTER_E2E_WEB_PORT ?? 3111);

export default defineConfig({
  testDir: "./e2e",
  outputDir: "test-results/e2e",
  timeout: 60_000,
  expect: { timeout: 15_000 },
  fullyParallel: false,
  workers: 1, // the tests share the stub's state and reset it before each test
  reporter: [["list"], ["junit", { outputFile: "test-results/e2e-junit.xml" }]],
  use: { baseURL: `http://localhost:${WEB_PORT}`, trace: "retain-on-failure" },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"], viewport: { width: 1280, height: 900 } } }],
  webServer: [
    {
      command: `node e2e/stub-gateway.mjs ${STUB_PORT}`,
      url: `http://localhost:${STUB_PORT}/readyz`,
      reuseExistingServer: false,
    },
    {
      // A production build: what ships, and steadier than the dev server.
      command: `node scripts/sync-content.mjs && pnpm exec next build && pnpm exec next start --port ${WEB_PORT}`,
      url: `http://localhost:${WEB_PORT}`,
      env: { GATEWAY_URL: `http://localhost:${STUB_PORT}`, NEXT_DIST_DIR: ".next-e2e" },
      timeout: 300_000,
      reuseExistingServer: false,
    },
  ],
});
