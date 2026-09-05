-- +goose Up
CREATE TABLE background_jobs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_type      TEXT NOT NULL,
    payload       JSONB NOT NULL DEFAULT '{}',
    status        TEXT NOT NULL DEFAULT 'PENDING',
    attempts      INTEGER NOT NULL DEFAULT 0,
    max_attempts  INTEGER NOT NULL DEFAULT 5,
    available_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_at     TIMESTAMPTZ,
    locked_by     TEXT,
    last_error    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at  TIMESTAMPTZ,
    CONSTRAINT background_jobs_status_check CHECK (
        status IN ('PENDING', 'RUNNING', 'COMPLETED', 'FAILED', 'DEAD_LETTER')
    )
);

CREATE INDEX background_jobs_poll_idx ON background_jobs (status, available_at);
CREATE INDEX background_jobs_job_type_idx ON background_jobs (job_type);

-- +goose Down
DROP TABLE background_jobs;
