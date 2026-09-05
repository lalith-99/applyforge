-- +goose Up
CREATE TABLE applications (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    job_id             UUID NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    resume_version_id  UUID REFERENCES resume_versions (id) ON DELETE SET NULL,
    status             TEXT NOT NULL DEFAULT 'SAVED',
    match_score        INTEGER,
    notes              TEXT,
    next_action        TEXT,
    applied_at         TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT applications_status_check CHECK (
        status IN (
            'SAVED', 'READY_TO_APPLY', 'APPLIED', 'RECRUITER_SCREEN', 'ASSESSMENT',
            'TECHNICAL_INTERVIEW', 'FINAL_INTERVIEW', 'OFFER', 'REJECTED', 'WITHDRAWN'
        )
    )
);

CREATE UNIQUE INDEX applications_user_job_idx ON applications (user_id, job_id);
CREATE INDEX applications_user_id_idx ON applications (user_id);
CREATE INDEX applications_status_idx ON applications (status);

-- +goose Down
DROP TABLE applications;
