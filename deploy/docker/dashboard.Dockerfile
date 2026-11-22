# The dashboard (decision record 0016). Build context: dashboard/, after
# `node dashboard/scripts/sync-content.mjs` has copied the benchmark reports into
# dashboard/content/benchmarks (the results pages read them at build time).
FROM node:24-slim AS base
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm install -g pnpm@12.11.2
WORKDIR /app

FROM base AS deps
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

FROM base AS build
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN test -d content/benchmarks || (echo "run scripts/sync-content.mjs before building" && exit 1)
RUN pnpm build

FROM node:24-slim AS runtime
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    HOSTNAME=0.0.0.0 \
    PORT=3000
WORKDIR /app
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static
COPY --from=build /app/public ./public
COPY --from=build /app/content ./content
# The node image's "node" user by number, so Kubernetes runAsNonRoot can verify it.
USER 1000:1000
EXPOSE 3000
CMD ["node", "server.js"]
