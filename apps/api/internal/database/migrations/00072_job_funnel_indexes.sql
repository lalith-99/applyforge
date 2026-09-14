-- +goose Up
-- Keep the high-volume ingestion and recommendation hot paths index-backed.
--
-- ListJobs/SearchJobsByEmbedding always start from active, canonical software
-- jobs and then constrain country/employment/freshness. The original schema has
-- separate status and posted_at indexes, which forces PostgreSQL to combine or
-- scan more rows as the catalog grows. This partial composite index mirrors the
-- stable hard-filter prefix while staying small by excluding closed, duplicate,
-- and non-software rows.
CREATE INDEX jobs_recommendation_funnel_idx
    ON jobs (country_code, employment_type, posted_at DESC, remote_type)
    INCLUDE (company_id, explicit_sponsorship_denied, explicit_sponsorship_supported)
    WHERE status = 'ACTIVE'
      AND canonical_job_id IS NULL
      AND role_classification = 'IC_SOFTWARE'
      AND posted_at IS NOT NULL;

-- Full-list ATS sources close postings by source/company/last_seen_at after a
-- successful poll. A composite partial index prevents lifecycle cleanup from
-- degrading into scans as direct Workday/iCIMS/SuccessFactors ingestion grows.
CREATE INDEX jobs_active_source_lifecycle_idx
    ON jobs (source, company_id, last_seen_at)
    WHERE status = 'ACTIVE';

-- Cross-source dedupe probes the fingerprint among active canonical rows on
-- every ingest. Keep that lookup bounded to the rows that can actually become
-- a canonical target.
CREATE INDEX jobs_active_canonical_fingerprint_idx
    ON jobs (fingerprint)
    WHERE status = 'ACTIVE'
      AND canonical_job_id IS NULL
      AND fingerprint <> '';

-- +goose Down
DROP INDEX IF EXISTS jobs_active_canonical_fingerprint_idx;
DROP INDEX IF EXISTS jobs_active_source_lifecycle_idx;
DROP INDEX IF EXISTS jobs_recommendation_funnel_idx;
