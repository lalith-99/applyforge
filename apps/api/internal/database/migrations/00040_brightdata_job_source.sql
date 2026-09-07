-- +goose Up
ALTER TABLE job_sources DROP CONSTRAINT job_sources_source_type_check;
ALTER TABLE job_sources ADD CONSTRAINT job_sources_source_type_check
    CHECK (source_type IN (
        'GREENHOUSE',
        'LEVER',
        'ASHBY',
        'ARBEITNOW',
        'SMARTRECRUITERS',
        'WORKABLE',
        'BRIGHTDATA'
    ));

INSERT INTO companies (name, normalized_name)
VALUES ('Bright Data Jobs', 'bright data jobs')
ON CONFLICT (normalized_name) DO NOTHING;

-- Disabled by default: enabling this source without BRIGHTDATA_API_KEY and
-- BRIGHTDATA_JOBS_DATASET_ID configured would only generate failed poll runs.
INSERT INTO job_sources (source_type, company_id, board_token, enabled)
SELECT 'BRIGHTDATA', id, 'us-software-24h', false
FROM companies
WHERE normalized_name = 'bright data jobs'
  AND NOT EXISTS (
      SELECT 1 FROM job_sources
      WHERE source_type = 'BRIGHTDATA' AND board_token = 'us-software-24h'
  );

-- +goose Down
DELETE FROM job_sources WHERE source_type = 'BRIGHTDATA';

ALTER TABLE job_sources DROP CONSTRAINT job_sources_source_type_check;
ALTER TABLE job_sources ADD CONSTRAINT job_sources_source_type_check
    CHECK (source_type IN (
        'GREENHOUSE',
        'LEVER',
        'ASHBY',
        'ARBEITNOW',
        'SMARTRECRUITERS',
        'WORKABLE'
    ));
