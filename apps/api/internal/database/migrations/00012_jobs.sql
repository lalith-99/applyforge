-- +goose Up
CREATE TABLE jobs (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source           TEXT NOT NULL,
    external_id      TEXT NOT NULL,
    company_id       UUID REFERENCES companies (id) ON DELETE SET NULL,
    company_name     TEXT NOT NULL,
    title            TEXT NOT NULL,
    normalized_title TEXT NOT NULL,
    seniority        TEXT,
    description      TEXT NOT NULL DEFAULT '',
    country          TEXT,
    state            TEXT,
    city             TEXT,
    location_text    TEXT,
    remote_type      TEXT,
    employment_type  TEXT,
    salary_min       INTEGER,
    salary_max       INTEGER,
    salary_currency  TEXT,
    apply_url        TEXT,
    source_url       TEXT,
    posted_at        TIMESTAMPTZ,
    first_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    content_hash     TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT jobs_status_check CHECK (status IN ('ACTIVE', 'CLOSED'))
);

CREATE UNIQUE INDEX jobs_source_external_id_idx ON jobs (source, external_id);
CREATE INDEX jobs_posted_at_idx ON jobs (posted_at DESC);
CREATE INDEX jobs_normalized_title_idx ON jobs (normalized_title);
CREATE INDEX jobs_company_id_idx ON jobs (company_id);
CREATE INDEX jobs_status_idx ON jobs (status);

-- +goose Down
DROP TABLE jobs;
