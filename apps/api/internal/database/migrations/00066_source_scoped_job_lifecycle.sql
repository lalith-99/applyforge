-- +goose Up
-- A job source is a concrete board/tenant, while jobs.source is only the
-- provider family (GREENHOUSE, LEVER, ...). Track membership at the concrete
-- source boundary so sibling boards can never close one another's jobs.
ALTER TABLE job_sources
    ADD COLUMN poll_generation BIGINT NOT NULL DEFAULT 0;

ALTER TABLE job_source_poll_runs
    ADD COLUMN poll_generation BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN snapshot_complete BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE job_source_postings (
    job_source_id UUID NOT NULL REFERENCES job_sources (id) ON DELETE CASCADE,
    external_id   TEXT NOT NULL,
    job_id        UUID NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    active        BOOLEAN NOT NULL DEFAULT true,
    PRIMARY KEY (job_source_id, external_id)
);

CREATE INDEX job_source_postings_job_active_idx
    ON job_source_postings (job_id, active);
CREATE INDEX job_source_postings_source_active_seen_idx
    ON job_source_postings (job_source_id, active, last_seen_at);

-- Existing rows can be attributed safely only when a company has exactly one
-- configured board for that provider. Ambiguous legacy rows remain untouched
-- and therefore cannot be destructively closed by an arbitrary sibling board.
WITH unambiguous_sources AS (
    SELECT source_type, company_id, min(id::text)::uuid AS job_source_id
    FROM job_sources
    GROUP BY source_type, company_id
    HAVING count(*) = 1
)
INSERT INTO job_source_postings (
    job_source_id, external_id, job_id, first_seen_at, last_seen_at, active
)
SELECT
    source.job_source_id,
    job.external_id,
    job.id,
    job.first_seen_at,
    job.last_seen_at,
    job.status = 'ACTIVE'
FROM jobs job
JOIN unambiguous_sources source
  ON source.source_type = job.source
 AND source.company_id = job.company_id
ON CONFLICT (job_source_id, external_id) DO NOTHING;

-- +goose Down
DROP TABLE job_source_postings;
ALTER TABLE job_source_poll_runs
    DROP COLUMN snapshot_complete,
    DROP COLUMN poll_generation;
ALTER TABLE job_sources
    DROP COLUMN poll_generation;
