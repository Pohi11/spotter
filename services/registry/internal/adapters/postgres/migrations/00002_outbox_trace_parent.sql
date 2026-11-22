-- +goose Up

-- The W3C traceparent active when the change was written; the relay attaches it when it
-- publishes, so the event belongs to the trace of the request that caused it.
ALTER TABLE outbox ADD COLUMN trace_parent text NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE outbox DROP COLUMN trace_parent;
