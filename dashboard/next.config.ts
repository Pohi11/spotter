import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Emits a self-contained server (.next/standalone) for a small image.
  output: "standalone",
  // The browser tests run their own dev server next to a developer's, so each gets its
  // own build folder.
  distDir: process.env.NEXT_DIST_DIR ?? ".next",
};

export default nextConfig;
