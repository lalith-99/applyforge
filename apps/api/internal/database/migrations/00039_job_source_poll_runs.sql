-- +goose Up
CREATE TABLE job_source_poll_runs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_source_id UUID NOT NULL REFERENCES job_sources (id) ON DELETE CASCADE,
    source_type  TEXT NOT NULL,
    board_token  TEXT NOT NULL,
    company_name TEXT NOT NULL,
    started_at   TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL,
    duration_ms  INTEGER NOT NULL DEFAULT 0,
    status       TEXT NOT NULL,
    fetched      INTEGER NOT NULL DEFAULT 0,
    inserted     INTEGER NOT NULL DEFAULT 0,
    updated      INTEGER NOT NULL DEFAULT 0,
    deduped      INTEGER NOT NULL DEFAULT 0,
    closed       INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    CONSTRAINT job_source_poll_runs_status_check CHECK (status IN ('SUCCESS', 'ERROR'))
);

CREATE INDEX job_source_poll_runs_source_started_idx
    ON job_source_poll_runs (job_source_id, started_at DESC);
CREATE INDEX job_source_poll_runs_started_idx
    ON job_source_poll_runs (started_at DESC);

-- +goose Down
DROP TABLE job_source_poll_runs;
