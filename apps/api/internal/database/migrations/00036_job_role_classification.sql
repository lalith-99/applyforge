-- +goose Up
ALTER TABLE jobs
    ADD COLUMN job_family TEXT NOT NULL DEFAULT 'UNKNOWN',
    ADD COLUMN role_classification TEXT NOT NULL DEFAULT 'UNKNOWN',
    ADD COLUMN role_classification_confidence REAL NOT NULL DEFAULT 0;

CREATE INDEX jobs_active_catalog_idx ON jobs (country_code, job_family, role_classification, posted_at DESC)
    WHERE status = 'ACTIVE' AND canonical_job_id IS NULL;

-- +goose Down
DROP INDEX jobs_active_catalog_idx;
ALTER TABLE jobs
    DROP COLUMN role_classification_confidence,
    DROP COLUMN role_classification,
    DROP COLUMN job_family;