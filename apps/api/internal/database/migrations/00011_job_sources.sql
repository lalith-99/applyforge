-- +goose Up
CREATE TABLE job_sources (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_type     TEXT NOT NULL,
    company_id      UUID NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    board_token     TEXT NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    last_polled_at  TIMESTAMPTZ,
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT job_sources_source_type_check CHECK (source_type IN ('GREENHOUSE', 'LEVER', 'ASHBY'))
);

CREATE UNIQUE INDEX job_sources_type_token_idx ON job_sources (source_type, board_token);

-- +goose Down
DROP TABLE job_sources;
