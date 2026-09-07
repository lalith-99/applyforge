-- +goose Up
-- Sponsor-first market discovery.
--
-- 1. Remove retired per-company ATS source rows. Fresh databases no longer
--    create them at all; this cleanup handles local databases created before
--    that change.
-- 2. Close legacy direct-source jobs so they cannot dominate a market-wide
--    local catalog after upgrading without a full reset.
-- 3. Provide one conservative database predicate for recent H-1B history.
--    Current job-posting "no sponsorship" text remains a separate, stronger
--    role-level exclusion.
DELETE FROM job_sources
WHERE source_type IN ('GREENHOUSE', 'LEVER', 'ASHBY', 'SMARTRECRUITERS', 'WORKABLE');

UPDATE jobs
SET status = 'CLOSED',
    updated_at = now()
WHERE status = 'ACTIVE'
  AND source IN ('GREENHOUSE', 'LEVER', 'ASHBY', 'SMARTRECRUITERS', 'WORKABLE');

DELETE FROM companies c
WHERE NOT EXISTS (SELECT 1 FROM jobs j WHERE j.company_id = c.id)
  AND NOT EXISTS (SELECT 1 FROM job_sources js WHERE js.company_id = c.id);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION company_has_recent_h1b_history(target_company_id UUID)
RETURNS BOOLEAN
LANGUAGE SQL
STABLE
AS $$
    WITH employer_keys AS (
        SELECT normalized_name AS employer_key
        FROM companies
        WHERE id = target_company_id

        UNION

        SELECT evidence_employer_normalized_name
        FROM company_immigration_aliases
        WHERE company_id = target_company_id
    ),
    current_fiscal_year AS (
        SELECT (
            EXTRACT(YEAR FROM current_date)::int
            + CASE WHEN EXTRACT(MONTH FROM current_date)::int >= 10 THEN 1 ELSE 0 END
        ) AS fiscal_year
    )
    SELECT EXISTS (
        SELECT 1
        FROM company_immigration_evidence e
        CROSS JOIN current_fiscal_year fy
        WHERE e.program = 'LCA_H1B'
          AND e.certified_count > 0
          AND e.fiscal_year >= fy.fiscal_year - 2
          AND EXISTS (
              SELECT 1
              FROM employer_keys k
              WHERE e.employer_normalized_name = k.employer_key
                 OR (
                     length(k.employer_key) >= 6
                     AND e.employer_normalized_name LIKE k.employer_key || ' %'
                 )
                 OR (
                     length(e.employer_normalized_name) >= 6
                     AND k.employer_key LIKE e.employer_normalized_name || ' %'
                 )
          )
    );
$;
-- +goose StatementEnd

-- Add the newer role-family shards when upgrading an already-created local DB.
INSERT INTO job_sources (
    source_type, company_id, board_token, enabled, poll_interval_minutes
)
SELECT 'BRIGHTDATA', id, shard, false, 360
FROM companies
CROSS JOIN (VALUES
    ('us-enterprise-apps-24h'),
    ('us-consulting-engineering-24h')
) AS shards(shard)
WHERE normalized_name = 'bright data jobs'
ON CONFLICT (source_type, board_token) DO UPDATE
SET poll_interval_minutes = EXCLUDED.poll_interval_minutes;

INSERT INTO job_sources (
    source_type, company_id, board_token, enabled, poll_interval_minutes
)
SELECT 'SERPAPI_GOOGLE_JOBS', id, shard, false, 360
FROM companies
CROSS JOIN (VALUES
    ('us-enterprise-apps-24h'),
    ('us-consulting-engineering-24h')
) AS shards(shard)
WHERE normalized_name = 'google jobs discovery'
ON CONFLICT (source_type, board_token) DO UPDATE
SET poll_interval_minutes = EXCLUDED.poll_interval_minutes;

-- +goose Down
DROP FUNCTION IF EXISTS company_has_recent_h1b_history(UUID);

DELETE FROM job_sources
WHERE board_token IN ('us-enterprise-apps-24h', 'us-consulting-engineering-24h');
