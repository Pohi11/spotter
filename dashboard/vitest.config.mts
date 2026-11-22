import { fileURLToPath } from "node:url";

import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: {
    alias: { "@": fileURLToPath(new URL(".", import.meta.url)) },
  },
  test: {
    include: ["lib/**/*.test.ts", "tests/**/*.test.ts"],
    // Test titles carry their requirement tag; speccheck reads them from this report
    // (decision record 0017).
    reporters: ["default", ["junit", { outputFile: "test-results/junit.xml" }]],
  },
});
