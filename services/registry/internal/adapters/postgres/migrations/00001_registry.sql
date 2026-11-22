-- +goose Up

CREATE TABLE models (
    id          uuid PRIMARY KEY,
    name        text NOT NULL CHECK (name ~ '^[a-z][a-z0-9-]{2,62}$'),
    description text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL,
    CONSTRAINT models_name_key UNIQUE (name)
);

CREATE TABLE model_versions (
    id              uuid PRIMARY KEY,
    model_id        uuid NOT NULL REFERENCES models (id),
    version         integer NOT NULL CHECK (version > 0),
    artifact_uri    text NOT NULL CHECK (artifact_uri ~ '^s3://[^/]+/.+$'),
    artifact_sha256 text NOT NULL CHECK (artifact_sha256 ~ '^[0-9a-f]{64}$'),
    framework       text NOT NULL CHECK (framework IN ('pytorch', 'onnx')),
    metrics         jsonb NOT NULL DEFAULT '{}',
    stage           text NOT NULL CHECK (stage IN ('registered', 'validated', 'shadow', 'production', 'retired', 'rejected', 'rolled_back')),
    revision        bigint NOT NULL CHECK (revision > 0),
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    CONSTRAINT model_versions_model_version_key UNIQUE (model_id, version)
);

-- At most one production version per model, enforced by the database itself.
CREATE UNIQUE INDEX model_versions_one_production_per_model
    ON model_versions (model_id) WHERE stage = 'production';

CREATE TABLE stage_changes (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    version_id uuid NOT NULL REFERENCES model_versions (id),
    from_stage text NOT NULL,
    to_stage   text NOT NULL,
    reason     text NOT NULL CHECK (btrim(reason) <> ''),
    revision   bigint NOT NULL, -- the version's revision after the change
    changed_at timestamptz NOT NULL,
    -- A second guard against two transitions from the same revision.
    CONSTRAINT stage_changes_version_revision_key UNIQUE (version_id, revision)
);

-- Events waiting to be published; written in the same transaction as the change.
CREATE TABLE outbox (
    id           uuid PRIMARY KEY,
    event_type   text NOT NULL,
    aggregate_id uuid NOT NULL,
    payload      bytea NOT NULL,
    occurred_at  timestamptz NOT NULL,
    published_at timestamptz
);

CREATE INDEX outbox_unpublished ON outbox (occurred_at) WHERE published_at IS NULL;

-- +goose Down

DROP TABLE outbox;
DROP TABLE stage_changes;
DROP TABLE model_versions;
DROP TABLE models;
