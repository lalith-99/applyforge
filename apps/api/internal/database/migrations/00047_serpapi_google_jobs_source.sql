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
        'BRIGHTDATA',
        'SERPAPI_GOOGLE_JOBS'
    ));

INSERT INTO companies (name, normalized_name)
VALUES ('Google Jobs Discovery', 'google jobs discovery')
ON CONFLICT (normalized_name) DO NOTHING;

INSERT INTO job_sources (
    source_type, company_id, board_token, enabled, poll_interval_minutes
)
SELECT 'SERPAPI_GOOGLE_JOBS', id, shard, false, 360
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
WHERE normalized_name = 'google jobs discovery'
ON CONFLICT (source_type, board_token) DO UPDATE
SET poll_interval_minutes = EXCLUDED.poll_interval_minutes;

-- +goose Down
DELETE FROM job_sources WHERE source_type = 'SERPAPI_GOOGLE_JOBS';
DELETE FROM companies WHERE normalized_name = 'google jobs discovery';

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
