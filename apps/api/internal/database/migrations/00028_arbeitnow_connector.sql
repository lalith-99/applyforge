-- +goose Up
-- Adds the Arbeitnow connector: a genuine multi-company job-search
-- aggregator (unlike Greenhouse/Lever/Ashby, one feed covers many
-- companies without needing a per-company board_token). See
-- internal/jobs/arbeitnow.go.
ALTER TABLE job_sources DROP CONSTRAINT job_sources_source_type_check;
ALTER TABLE job_sources ADD CONSTRAINT job_sources_source_type_check
    CHECK (source_type IN ('GREENHOUSE', 'LEVER', 'ASHBY', 'ARBEITNOW'));

-- job_sources.company_id is NOT NULL, but Arbeitnow jobs carry their own
-- per-job company (resolved dynamically in Ingest) - this placeholder
-- company just satisfies the FK for the single ARBEITNOW job_sources row.
INSERT INTO companies (name, normalized_name) VALUES ('Arbeitnow Aggregator', 'arbeitnow-aggregator');

INSERT INTO job_sources (source_type, company_id, board_token)
SELECT 'ARBEITNOW', id, 'global' FROM companies WHERE normalized_name = 'arbeitnow-aggregator';

-- +goose Down
DELETE FROM job_sources WHERE source_type = 'ARBEITNOW';
DELETE FROM companies WHERE normalized_name = 'arbeitnow-aggregator';
ALTER TABLE job_sources DROP CONSTRAINT job_sources_source_type_check;
ALTER TABLE job_sources ADD CONSTRAINT job_sources_source_type_check
    CHECK (source_type IN ('GREENHOUSE', 'LEVER', 'ASHBY'));
