-- +goose Up
CREATE TABLE resume_experiences (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resume_id       UUID NOT NULL REFERENCES resumes (id) ON DELETE CASCADE,
    display_order   INTEGER NOT NULL DEFAULT 0,
    company         TEXT,
    title           TEXT,
    start_date      TEXT,
    end_date        TEXT,
    location        TEXT,
    bullets          TEXT[] NOT NULL DEFAULT '{}',
    detected_skills TEXT[] NOT NULL DEFAULT '{}',
    technologies    TEXT[] NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX resume_experiences_resume_id_idx ON resume_experiences (resume_id);

-- +goose Down
DROP TABLE resume_experiences;
