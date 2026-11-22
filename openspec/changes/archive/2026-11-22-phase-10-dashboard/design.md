# Design

## Context

The platform's state lives in three places: Postgres behind the registry (versions, stages, gates), JetStream (every event, the job bucket), and Prometheus (metrics). The gateway is the only public front door and already serves REST for the registry and the inference API (changes `phase-02-registry-api`, `phase-04-event-driven-inference`). The dashboard reads and acts through the same front door, so it needs no new credentials and no direct access to Postgres or NATS. The choices are recorded in decision record [0016](../../../../docs/decisions/0016-dashboard-and-gateway-event-feed.md); TypeScript tests name their requirement as described in [0017](../../../../docs/decisions/0017-typescript-spec-test-traceability.md).

## Decisions

### D1. The event feed lives in the gateway
`GET /v1/events` is a plain `net/http` handler on the gateway's HTTP mux (not a grpc-gateway route; server-sent events aren't a protobuf RPC). The handler depends on a port, `ports.EventSource`, with one method that returns a channel of `domain.Event` values for a replay window; the adapter `adapters/natsevents` implements it with one ordered consumer per stream (`DeliverByStartTimePolicy` at now minus `since`). Ordered consumers are ephemeral and in-memory, so a disconnect or a crash leaves no durable state, and each client sees each stream in order. The route sits outside the `otelhttp` wrapper so an hours-long stream isn't one span.

### D2. Payloads are decoded by type name
Every publisher sets `Spotter-Event-Type` to the protobuf message's full name. The adapter looks the name up in the global protobuf registry (all `spotter.v1` types are linked into the gateway) and renders it with `protojson`. An unknown or missing type keeps the envelope and sends a null payload rather than dropping the event. No event carries image bytes: `InferenceRequested` holds only the image's URI.

### D3. Drift evaluations become events
The monitor's detector already returns every feature's result at each evaluation. The use case now publishes those results as `DriftEvaluated` with a de-duplication id built from the version id and the stream sequence of the completion that triggered it, the same scheme as the state-change events. This adds one small message per 50 completions per version.

### D4. Dashboard shape
`dashboard/` is a Next.js App Router app with TypeScript and Tailwind CSS. Server components fetch registry data from `GATEWAY_URL` while rendering. Client components (the classify panel, the live list, the drift chart, the model actions) call `/api/...`, a route handler that forwards an allowlist of method and path patterns to the gateway and streams the response body through, which also carries the event feed. The allowlist is a pure function with unit tests.

### D5. Content that isn't fetched live is generated
- **Results:** a build-time loader reads `benchmarks/*.json` (copied into `dashboard/content/benchmarks/` by `pnpm sync`), so every number on the results page comes from a dated report.
- **Worked example:** `dashboard/content/worked-example.json` is recorded from a real job by `pnpm record-example` (the job, its completion event, and the span timings from Jaeger).
- **Samples:** `spotter-demo-samples` (Python, trainer package) writes holdout tiles and their degradations as PNG files with a manifest into `dashboard/public/samples/`, using `spotter-degrade`'s functions so the panel's degradations are the experiment's.

### D6. Visual language
One typeface, Poppins, for text and headings, with the system monospace face only for code identifiers. A cool grey-blue page with navy ink. Colors carry fixed meanings drawn from map conventions, never decoration: water blue for Go services and code, green for production and passed gates, ochre for shadow and pending, red for drift, failures, and rollback, violet for the model and the Python worker, and teal for the event bus. The home page opens with a contact sheet of real classified tiles; everything else is quiet.

### D7. Testing
- Go: the event feed against an embedded JetStream (`natstest`), including disconnect cleanup and the keep-alive (interval injectable); the monitor use case with fakes.
- TypeScript: Vitest for the allowlist, event summaries, drift series, gate logic, and the results loader.
- Browser: Playwright against the dashboard in front of a stub gateway (`e2e/stub-gateway.ts`) that serves JSON recorded from the live gateway and replays an event stream. CI runs these without the platform. One run against the real stack is recorded in the change's results.
- Python: the sample export.

## Risks and trade-offs

- **No authentication:** anyone who can open the dashboard can approve, promote, and roll back. Acceptable for a laptop or a demo cluster; a shared deployment needs an identity provider in front of it, and the approver name would then come from the identity, not a text box.
- **Each feed client holds three consumers on the server.** Fine for a handful of viewers; many viewers would need a fan-out in the gateway.
- **Recorded fixtures can drift from the real API.** The fixtures are re-recorded by a script against a running gateway, and the protobuf JSON shape is guarded by `buf breaking`.
