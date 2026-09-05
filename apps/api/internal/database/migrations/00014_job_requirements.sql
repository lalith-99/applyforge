-- +goose Up
CREATE TABLE job_requirements (
    id                               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id                           UUID NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    content_hash                     TEXT NOT NULL,
    role_family                      TEXT,
    normalized_title                 TEXT,
    seniority                        TEXT,
    required_skills                  JSONB NOT NULL DEFAULT '[]',
    preferred_skills                 JSONB NOT NULL DEFAULT '[]',
    required_experience_years        INTEGER,
    responsibilities                 JSONB NOT NULL DEFAULT '[]',
    domains                          TEXT[] NOT NULL DEFAULT '{}',
    education_requirements           TEXT[] NOT NULL DEFAULT '{}',
    certifications                   TEXT[] NOT NULL DEFAULT '{}',
    location_requirements            TEXT,
    employment_type                  TEXT,
    clearance_requirements           TEXT,
    work_authorization_requirements  TEXT,
    keywords                         TEXT[] NOT NULL DEFAULT '{}',
    parsed_at                        TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at                       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX job_requirements_job_id_idx ON job_requirements (job_id);

-- +goose Down
DROP TABLE job_requirements;
