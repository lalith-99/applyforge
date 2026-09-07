-- +goose Up
ALTER TABLE job_sources
    ADD COLUMN poll_interval_minutes INTEGER NOT NULL DEFAULT 60,
    ADD CONSTRAINT job_sources_poll_interval_check
        CHECK (poll_interval_minutes BETWEEN 15 AND 1440);

-- Direct ATS sources remain hourly. Aggregators and paid broad discovery run
-- less frequently to control provider cost and avoid wasteful repeated pulls.
UPDATE job_sources SET poll_interval_minutes = 120
WHERE source_type = 'ARBEITNOW';

UPDATE job_sources SET poll_interval_minutes = 360
WHERE source_type = 'BRIGHTDATA';

-- Replace the legacy monolithic Bright Data search with bounded stack shards.
UPDATE job_sources
SET enabled = false
WHERE source_type = 'BRIGHTDATA' AND board_token = 'us-software-24h';

INSERT INTO job_sources (
    source_type, company_id, board_token, enabled, poll_interval_minutes
)
SELECT 'BRIGHTDATA', id, shard, false, 360
FROM companies
CROSS JOIN (VALUES
    ('us-software-general-24h'),
    ('us-java-24h'),
    ('us-go-24h'),
    ('us-fullstack-web-24h'),
    ('us-backend-platform-24h'),
    ('us-devops-cloud-24h'),
    ('us-data-ai-24h'),
    ('us-language-developers-24h')
) AS shards(shard)
WHERE normalized_name = 'bright data jobs'
ON CONFLICT (source_type, board_token) DO UPDATE
SET poll_interval_minutes = EXCLUDED.poll_interval_minutes;

-- +goose Down
DELETE FROM job_sources
WHERE source_type = 'BRIGHTDATA'
  AND board_token IN (
      'us-software-general-24h',
      'us-java-24h',
      'us-go-24h',
      'us-fullstack-web-24h',
      'us-backend-platform-24h',
      'us-devops-cloud-24h',
      'us-data-ai-24h',
      'us-language-developers-24h'
  );

UPDATE job_sources
SET poll_interval_minutes = 60
WHERE source_type IN ('ARBEITNOW', 'BRIGHTDATA');

ALTER TABLE job_sources DROP CONSTRAINT job_sources_poll_interval_check;
ALTER TABLE job_sources DROP COLUMN poll_interval_minutes;
