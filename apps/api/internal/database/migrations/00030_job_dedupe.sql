-- +goose Up
-- Cross-source deduplication: same job posted via two different sources
-- (e.g. a company's own Greenhouse board AND an aggregator like Arbeitnow)
-- previously created two separate job rows with no user-visible link. Rather
-- than splitting jobs into job_postings/canonical_jobs (a much larger,
-- riskier schema+query rewrite touching matching/tailoring/frontend), a
-- self-referencing canonical_job_id keeps the existing jobs table as the
-- single source of truth: a duplicate row points at the canonical row that
-- represents the same real-world opportunity, and listing queries simply
-- exclude non-canonical rows.
ALTER TABLE jobs ADD COLUMN fingerprint TEXT NOT NULL DEFAULT '';
ALTER TABLE jobs ADD COLUMN canonical_job_id UUID REFERENCES jobs (id) ON DELETE SET NULL;

CREATE INDEX jobs_fingerprint_idx ON jobs (fingerprint) WHERE canonical_job_id IS NULL;
CREATE INDEX jobs_canonical_job_id_idx ON jobs (canonical_job_id);

-- +goose Down
DROP INDEX jobs_canonical_job_id_idx;
DROP INDEX jobs_fingerprint_idx;
ALTER TABLE jobs DROP COLUMN canonical_job_id;
ALTER TABLE jobs DROP COLUMN fingerprint;
