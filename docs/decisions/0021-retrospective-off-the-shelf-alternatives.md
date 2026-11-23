# 0021 — Retrospective: what off-the-shelf tools would have given this design, and cost

- **Date:** 2026-11-22
- **Status:** Accepted. Records no change; it compares what was built with what could have been adopted.

## Context

Spotter builds its registry, serving path, monitoring, promotion, and messaging from small parts: Go services, a Python worker, NATS JetStream, Postgres, and an S3-compatible store. Each part has a mature off-the-shelf alternative. This record weighs, with hindsight and the measurements in `benchmarks/`, what each alternative would have given and what it would have cost, so the next design starts from evidence rather than habit.

## Options considered

### Model serving: KServe, NVIDIA Triton, BentoML

- **KServe** (Kubernetes CRDs over Knative or raw Deployments): standard inference protocol, canary and shadow traffic splitting at the network layer, scale-to-zero, model storage initializers for S3. It would replace the worker's model loading, the shadow slot, and part of the promotion swap. Cost: Knative or Istio to operate, and canary routing that splits traffic rather than shadowing every request; agreement statistics, the gates, and the human approval would still be custom. The measured promotion and rollback times (232 ms and 457 ms to the worker serving the new version) come from an event and a reload; KServe would need a rollout instead.
- **Triton Inference Server**: dynamic batching, concurrent model execution, ONNX Runtime, TensorRT, and PyTorch backends, and metrics out of the box. For a 289-thousand-parameter CNN taking about 2 ms per image, batching gains little end to end: the load tests found the model was 3 ms of a 65 ms job, with the object-store write the largest cost. Triton matters when the model dominates; here it would not have moved the measured bottleneck. It also has no notion of an event bus; the worker would still consume NATS and call Triton.
- **BentoML**: packaging, an HTTP and gRPC server, adaptive batching, and a model store in one Python framework. It would replace the worker's serving code and artifact handling, but the registry, its stage machine, and the gates would stay outside it, and its model store overlaps the registry.

### Experiment tracking and registry: MLflow

MLflow's tracking server records parameters, metrics, and artifacts per run, and its model registry has versions, aliases, and stage tags. Spotter's training metadata (`metadata.json` beside a content-addressed artifact) and the registry cover the same ground. MLflow would have saved the metadata format and given a UI for runs. It would not give what the registry enforces: a transition table that refuses illegal moves, one production version per model in the database, optimistic concurrency, a transactional outbox, and gates as code. Those were the point of the registry, and they would have been built as a service beside MLflow anyway.

### Messaging: Kafka and RabbitMQ instead of NATS JetStream

- **Kafka**: partitioned logs with consumer groups, very high throughput, long retention, and a large ecosystem (Connect, Schema Registry). Ordering is per partition, so per-job work would need keys; there is no built-in per-message acknowledgment with redelivery after a timeout, so a slow or crashed consumer blocks its partition until it commits or is rebalanced. Exactly-once semantics exist inside Kafka transactions, not across a database and the bus, so the outbox pattern would remain. Cost: a heavier cluster (KRaft controllers and brokers) for a platform whose measured peak was 189 jobs per second.
- **RabbitMQ**: work queues with per-message acknowledgment and redelivery fit the job path well; quorum queues give durability. It has no first-class key-value store, so job state would move to Postgres or Redis, and replaying the last day of events for the dashboard and the drift charts would need streams (RabbitMQ Streams) or a second system.
- **NATS JetStream**, as built: per-message acknowledgment and redelivery (the worker-kill measurements in `benchmarks/` exercise them), de-duplication by message id, a key-value bucket with compare-and-set for job state, replay by start time for the dashboard, and a single small binary that also runs at the edge. Its gaps showed up too: the de-duplication window is two minutes, so a much later re-publish would be stored twice, and KEDA scales on consumer lag by polling NATS's monitoring endpoint.

### Workflows: Temporal

Temporal would model "drift detected, retrain, shadow, wait for gates and an approval, promote" as one durable workflow with timers and signals, with retries and history built in. Spotter spreads that across events: the monitor publishes drift, the retrainer consumes it, the registry holds the stages, and a person acts in the dashboard. Temporal would make the long-running process explicit and inspectable in one place. Cost: a Temporal cluster (and its database) to operate, and workflow code with determinism rules; for a loop that runs a few times a month, events and a state machine in Postgres were enough.

## Decision

Keep the design as built for this scope, and adopt the alternatives at these thresholds:

- **Triton or KServe** when the model's own time dominates a job (larger models, GPUs), or when traffic splitting across clusters is needed.
- **MLflow** when several people train many runs and need to compare them; keep the registry service as the authority for stages and gates.
- **Kafka** when event volume or retention outgrows JetStream, or when many independent consumers need long replay; keep the outbox.
- **Temporal** when promotion grows more steps, waits, and people than a state machine and a few events express clearly.

## Consequences

- **Good:** Every guarantee in the platform is code in this repository with a test that names it, which is why each could be measured under faults.
- **Cost:** Features those tools give for free were built by hand or left out: dynamic batching, traffic-split canaries, a run-comparison UI, and long-horizon event retention.
- **Cost:** Operating knowledge transfers less directly: an organization standardized on KServe and MLflow would need the registry's rules mapped onto their concepts.
