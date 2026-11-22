# 0009 — Inference jobs: at-least-once delivery, KV job state, one result per job

- **Date:** 2026-11-22
- **Status:** Accepted

## Context

Inference requests travel over NATS JetStream from the gateway to Python workers. Workers can crash or be stopped mid-job (Kubernetes reschedules pods; a node can disappear). Every job must eventually finish, and no job may end up with two different results or two completion events that downstream consumers (the drift monitor) would double-count. The gateway also needs somewhere to keep each job's status so clients can read it.

## Options considered

**Delivery**
1. **Core NATS request/reply.** Simple and fast, but nothing survives a worker crash; the client just times out.
2. **JetStream with at-least-once delivery** (explicit ack, redelivery after an ack wait), made safe by idempotent processing.
3. **"Exactly-once" by JetStream's double-ack plus de-duplication alone.** De-duplication covers publishes within a time window, not processing; a worker can still classify the same job twice.

**Where job status lives**
4. **A Postgres schema owned by the gateway.** Durable and queryable, but a second database schema, migrations, and connection pool just for short-lived job status.
5. **JetStream key-value bucket.** Already running, has per-key revisions for compare-and-set, and a TTL for expiry.
6. **Results only as events; the gateway materializes them.** Adds a consumer to the gateway and still needs storage.

## Decision

Options 2 and 5.

- Requests, completions, and dead letters are JetStream messages whose message ids are derived from the job id with a prefix per kind (`requested-`, `completed-`, `failed-`), so the stream drops a re-publish of the same event within two minutes. The prefix matters: all three share the `INFERENCE` stream and its duplicate window, and with the bare job id the completion was silently dropped as a duplicate of the request (found by the first integration test).
- Job status is a KV entry per job: the gateway creates it as `PENDING`; a worker moves it to `SUCCEEDED` or `FAILED` only with the revision it read (compare-and-set).
- A worker acknowledges only after the result is recorded and the completion is published. A redelivered job that is already final is acknowledged after re-publishing its completion.
- Transient failures are retried with a delay; poison messages and exhausted retries are recorded as `FAILED` and published to `inference.failed.<model>`.

## Consequences

- **Good:** A crash at any point leaves the job either still pending (it will be redelivered) or finished with exactly one result.
- **Good:** No new database for the gateway; KV's compare-and-set gives the "only if still pending" check directly.
- **Cost:** A job can be classified twice (wasted compute) when a worker crashes after classifying; only one result is kept.
- **Cost:** De-duplication is time-bounded; downstream consumers still key on job id rather than trusting the stream to be duplicate-free forever.
- **Cost:** Job status expires after 24 hours.
