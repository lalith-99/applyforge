-- +goose Up
-- Single-user Bright Data MVP shard.
--
-- The previous broad multi-shard configuration is retained for future
-- scale-out but stays disabled. The API explicitly enables only
-- BRIGHTDATA_ACTIVE_SHARDS.
INSERT INTO job_sources (
    source_type, company_id, board_token, enabled, poll_interval_minutes
)
SELECT 'BRIGHTDATA', id, 'us-single-user-core-24h', false, 1440
FROM companies
WHERE normalized_name = 'bright data jobs'
ON CONFLICT (source_type, board_token) DO UPDATE
SET enabled = false,
    poll_interval_minutes = EXCLUDED.poll_interval_minutes;

UPDATE job_sources
SET enabled = false
WHERE source_type = 'BRIGHTDATA'
  AND board_token <> 'us-single-user-core-24h';

-- +goose Down
DELETE FROM job_sources
WHERE source_type = 'BRIGHTDATA'
  AND board_token = 'us-single-user-core-24h';
