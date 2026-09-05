-- +goose Up
CREATE TABLE resume_versions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    base_resume_id  UUID NOT NULL REFERENCES resumes (id) ON DELETE CASCADE,
    job_id          UUID REFERENCES jobs (id) ON DELETE SET NULL,
    tailoring_run_id UUID REFERENCES tailoring_runs (id) ON DELETE SET NULL,
    version_number  INTEGER NOT NULL,
    content_json    JSONB NOT NULL,
    match_score     INTEGER,
    alignment_score INTEGER,
    tailoring_mode  TEXT,
    pdf_storage_key  TEXT,
    docx_storage_key TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX resume_versions_user_id_idx ON resume_versions (user_id);
CREATE INDEX resume_versions_base_resume_id_idx ON resume_versions (base_resume_id);
CREATE UNIQUE INDEX resume_versions_resume_version_idx ON resume_versions (base_resume_id, version_number);

-- +goose Down
DROP TABLE resume_versions;
