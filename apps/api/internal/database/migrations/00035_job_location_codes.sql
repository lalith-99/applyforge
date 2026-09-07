-- +goose Up
ALTER TABLE jobs
    ADD COLUMN country_code TEXT,
    ADD COLUMN state_code TEXT,
    ADD COLUMN workplace_type TEXT NOT NULL DEFAULT 'ONSITE',
    ADD COLUMN remote_scope TEXT NOT NULL DEFAULT 'UNKNOWN',
    ADD COLUMN eligible_country_codes TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN location_confidence TEXT NOT NULL DEFAULT 'LOW',
    ADD CONSTRAINT jobs_country_code_check CHECK (country_code IS NULL OR country_code ~ '^[A-Z]{2}$'),
    ADD CONSTRAINT jobs_workplace_type_check CHECK (workplace_type IN ('REMOTE', 'HYBRID', 'ONSITE')),
    ADD CONSTRAINT jobs_remote_scope_check CHECK (remote_scope IN ('US', 'WORLDWIDE', 'STATE_RESTRICTED', 'UNKNOWN')),
    ADD CONSTRAINT jobs_location_confidence_check CHECK (location_confidence IN ('HIGH', 'MEDIUM', 'LOW'));

CREATE INDEX jobs_active_country_posted_idx ON jobs (country_code, posted_at DESC)
    WHERE status = 'ACTIVE' AND canonical_job_id IS NULL;

-- Preserve existing, unambiguous U.S. rows without guessing from broad text
-- such as state abbreviations or an unscoped "remote" label.
UPDATE jobs
SET country_code = 'US',
        eligible_country_codes = ARRAY['US'],
        location_confidence = 'HIGH',
        remote_scope = CASE WHEN remote_type = 'remote' THEN 'US' ELSE remote_scope END
WHERE country_code IS NULL
    AND (
        lower(country) IN ('united states', 'united states of america', 'us', 'usa', 'u.s.', 'u.s')
        OR location_text ~* '(^|[^a-z])(united states|usa|u\\.s\\.?)([^a-z]|$)'
    );

-- +goose Down
DROP INDEX jobs_active_country_posted_idx;
ALTER TABLE jobs
    DROP CONSTRAINT jobs_location_confidence_check,
    DROP CONSTRAINT jobs_remote_scope_check,
    DROP CONSTRAINT jobs_workplace_type_check,
    DROP CONSTRAINT jobs_country_code_check,
    DROP COLUMN location_confidence,
    DROP COLUMN eligible_country_codes,
    DROP COLUMN remote_scope,
    DROP COLUMN workplace_type,
    DROP COLUMN state_code,
    DROP COLUMN country_code;