-- +goose Up
-- Separate original publication evidence from provider update timestamps.
-- posted_at remains the compatibility/read-model column and is populated only
-- from publication evidence after this migration.
ALTER TABLE jobs
    ADD COLUMN published_at TIMESTAMPTZ,
    ADD COLUMN source_updated_at TIMESTAMPTZ,
    ADD COLUMN date_kind TEXT NOT NULL DEFAULT 'UNKNOWN',
    ADD COLUMN date_precision TEXT NOT NULL DEFAULT 'UNKNOWN',
    ADD COLUMN date_source TEXT,
    ADD COLUMN explicit_sponsorship_supported BOOLEAN NOT NULL DEFAULT false,
    ADD CONSTRAINT jobs_date_kind_check
        CHECK (date_kind IN ('PUBLISHED', 'UPDATED', 'UNKNOWN')),
    ADD CONSTRAINT jobs_date_precision_check
        CHECK (date_precision IN ('SECOND', 'DAY', 'RELATIVE', 'UNKNOWN'));

-- Greenhouse's public Job Board payload exposes updated_at, not the original
-- publication time. Treating it as posted_at made edited old roles appear new.
UPDATE jobs
SET source_updated_at = posted_at,
    posted_at = NULL,
    date_kind = CASE WHEN posted_at IS NULL THEN 'UNKNOWN' ELSE 'UPDATED' END,
    date_precision = CASE WHEN posted_at IS NULL THEN 'UNKNOWN' ELSE 'SECOND' END,
    date_source = CASE WHEN posted_at IS NULL THEN NULL ELSE 'GREENHOUSE_UPDATED_AT' END
WHERE source = 'GREENHOUSE';

UPDATE jobs
SET published_at = posted_at,
    date_kind = CASE WHEN posted_at IS NULL THEN 'UNKNOWN' ELSE 'PUBLISHED' END,
    date_precision = CASE WHEN posted_at IS NULL THEN 'UNKNOWN' ELSE 'SECOND' END,
    date_source = CASE WHEN posted_at IS NULL THEN NULL ELSE source || '_LEGACY_POSTED_AT' END
WHERE source <> 'GREENHOUSE';

UPDATE jobs
SET explicit_sponsorship_supported = true
WHERE explicit_sponsorship_denied = false
  AND lower(description) LIKE ANY (ARRAY[
      '%h-1b sponsorship available%',
      '%h1b sponsorship available%',
      '%visa sponsorship available%',
      '%visa sponsorship provided%',
      '%sponsorship is available%',
      '%sponsorship available%',
      '%we sponsor h-1b%',
      '%we sponsor h1b%',
      '%sponsor h-1b%',
      '%sponsor h1b%',
      '%h-1b visa sponsorship%',
      '%h1b visa sponsorship%',
      '%h-1b transfer%',
      '%h1b transfer%',
      '%h-1b portability%',
      '%h1b portability%',
      '%support h-1b%',
      '%support h1b%',
      '%h-1b sponsorship support%',
      '%h1b sponsorship support%',
      '%provide visa sponsorship%',
      '%provides visa sponsorship%'
  ]);

CREATE INDEX jobs_published_at_idx ON jobs (published_at DESC);
CREATE INDEX jobs_active_h1b_evidence_idx
    ON jobs (country_code, role_classification, posted_at DESC)
    WHERE status = 'ACTIVE'
      AND canonical_job_id IS NULL
      AND explicit_sponsorship_denied = false
      AND explicit_sponsorship_supported = true;

-- +goose Down
DROP INDEX jobs_active_h1b_evidence_idx;
DROP INDEX jobs_published_at_idx;
ALTER TABLE jobs
    DROP CONSTRAINT jobs_date_precision_check,
    DROP CONSTRAINT jobs_date_kind_check,
    DROP COLUMN explicit_sponsorship_supported,
    DROP COLUMN date_source,
    DROP COLUMN date_precision,
    DROP COLUMN date_kind,
    DROP COLUMN source_updated_at,
    DROP COLUMN published_at;
