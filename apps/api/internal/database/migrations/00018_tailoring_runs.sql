-- +goose Up
CREATE TABLE tailoring_runs (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    job_id                 UUID NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    resume_id              UUID NOT NULL REFERENCES resumes (id) ON DELETE CASCADE,
    mode                   TEXT NOT NULL,
    status                 TEXT NOT NULL DEFAULT 'PENDING',
    summary_suggestion     JSONB,
    keyword_coverage       JSONB NOT NULL DEFAULT '{}',
    alignment_score_before INTEGER,
    alignment_score_after  INTEGER,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at           TIMESTAMPTZ,
    CONSTRAINT tailoring_runs_mode_check CHECK (mode IN ('STRICT', 'GROWTH', 'MAX_MATCH')),
    CONSTRAINT tailoring_runs_status_check CHECK (status IN ('PENDING', 'COMPLETED', 'FAILED'))
);

CREATE INDEX tailoring_runs_user_id_idx ON tailoring_runs (user_id);
CREATE INDEX tailoring_runs_job_id_idx ON tailoring_runs (job_id);

-- +goose Down
DROP TABLE tailoring_runs;
