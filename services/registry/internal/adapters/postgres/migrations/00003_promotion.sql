-- +goose Up

CREATE TABLE approvals (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    version_id  uuid NOT NULL REFERENCES model_versions (id),
    approver    text NOT NULL CHECK (btrim(approver) <> ''),
    reason      text NOT NULL CHECK (btrim(reason) <> ''),
    approved_at timestamptz NOT NULL
);

-- One row per job: a redelivered ShadowCompared event is ignored (ON CONFLICT DO NOTHING).
CREATE TABLE shadow_comparisons (
    version_id uuid NOT NULL REFERENCES model_versions (id),
    job_id     text NOT NULL,
    agree      boolean NOT NULL,
    shadow_ms  double precision NOT NULL CHECK (shadow_ms >= 0),
    PRIMARY KEY (version_id, job_id)
);

-- +goose Down

DROP TABLE shadow_comparisons;
DROP TABLE approvals;
