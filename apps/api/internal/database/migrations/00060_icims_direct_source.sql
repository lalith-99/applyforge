-- +goose Up
-- iCIMS became a verified direct connector after the structured source
-- registry was introduced. Keep the database source-type constraint and the
-- set-based sponsor-tier cadence machinery aligned with the Go connector set.

ALTER TABLE job_sources DROP CONSTRAINT job_sources_source_type_check;
ALTER TABLE job_sources ADD CONSTRAINT job_sources_source_type_check
    CHECK (source_type IN (
        'GREENHOUSE',
        'LEVER',
        'ASHBY',
        'ARBEITNOW',
        'SMARTRECRUITERS',
        'WORKABLE',
        'WORKDAY',
        'ICIMS',
        'CAREER_PAGE',
        'BRIGHTDATA',
        'SERPAPI_GOOGLE_JOBS'
    ));

-- Migration 00056 replaced the older per-row watchlist trigger with three
-- transition-table statement triggers. Extend those functions rather than
-- reviving the O(10k) row-trigger path.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION sync_sponsor_direct_source_cadence_insert_stmt()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE job_sources js
    SET poll_interval_minutes = n.poll_interval_minutes
    FROM sponsor_rows_new n
    WHERE js.company_id = n.company_id
      AND js.source_type IN (
          'GREENHOUSE',
          'LEVER',
          'ASHBY',
          'SMARTRECRUITERS',
          'WORKABLE',
          'WORKDAY',
          'ICIMS',
          'CAREER_PAGE'
      )
      AND js.poll_interval_minutes IS DISTINCT FROM n.poll_interval_minutes;

    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION sync_sponsor_direct_source_cadence_update_stmt()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE job_sources js
    SET poll_interval_minutes = n.poll_interval_minutes
    FROM sponsor_rows_new n
    JOIN sponsor_rows_old o USING (company_id)
    WHERE js.company_id = n.company_id
      AND n.poll_interval_minutes IS DISTINCT FROM o.poll_interval_minutes
      AND js.source_type IN (
          'GREENHOUSE',
          'LEVER',
          'ASHBY',
          'SMARTRECRUITERS',
          'WORKABLE',
          'WORKDAY',
          'ICIMS',
          'CAREER_PAGE'
      )
      AND js.poll_interval_minutes IS DISTINCT FROM n.poll_interval_minutes;

    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION sync_sponsor_direct_source_cadence_delete_stmt()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE job_sources js
    SET enabled = false
    FROM sponsor_rows_old o
    WHERE js.company_id = o.company_id
      AND js.source_type IN (
          'GREENHOUSE',
          'LEVER',
          'ASHBY',
          'SMARTRECRUITERS',
          'WORKABLE',
          'WORKDAY',
          'ICIMS',
          'CAREER_PAGE'
      )
      AND js.enabled = true;

    RETURN NULL;
END;
$$;
-- +goose StatementEnd

DROP INDEX IF EXISTS job_sources_company_direct_idx;
CREATE INDEX job_sources_company_direct_idx
    ON job_sources (company_id, source_type)
    WHERE source_type IN (
        'GREENHOUSE',
        'LEVER',
        'ASHBY',
        'SMARTRECRUITERS',
        'WORKABLE',
        'WORKDAY',
        'ICIMS',
        'CAREER_PAGE'
    );

-- +goose Down
-- Remove rows that would violate the restored pre-iCIMS constraint before
-- narrowing it. Registry candidates are intentionally preserved so upgrading
-- again can re-promote them without rediscovery.
DELETE FROM job_sources WHERE source_type = 'ICIMS';

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION sync_sponsor_direct_source_cadence_insert_stmt()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE job_sources js
    SET poll_interval_minutes = n.poll_interval_minutes
    FROM sponsor_rows_new n
    WHERE js.company_id = n.company_id
      AND js.source_type IN (
          'GREENHOUSE',
          'LEVER',
          'ASHBY',
          'SMARTRECRUITERS',
          'WORKABLE',
          'WORKDAY',
          'CAREER_PAGE'
      )
      AND js.poll_interval_minutes IS DISTINCT FROM n.poll_interval_minutes;

    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION sync_sponsor_direct_source_cadence_update_stmt()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE job_sources js
    SET poll_interval_minutes = n.poll_interval_minutes
    FROM sponsor_rows_new n
    JOIN sponsor_rows_old o USING (company_id)
    WHERE js.company_id = n.company_id
      AND n.poll_interval_minutes IS DISTINCT FROM o.poll_interval_minutes
      AND js.source_type IN (
          'GREENHOUSE',
          'LEVER',
          'ASHBY',
          'SMARTRECRUITERS',
          'WORKABLE',
          'WORKDAY',
          'CAREER_PAGE'
      )
      AND js.poll_interval_minutes IS DISTINCT FROM n.poll_interval_minutes;

    RETURN NULL;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION sync_sponsor_direct_source_cadence_delete_stmt()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE job_sources js
    SET enabled = false
    FROM sponsor_rows_old o
    WHERE js.company_id = o.company_id
      AND js.source_type IN (
          'GREENHOUSE',
          'LEVER',
          'ASHBY',
          'SMARTRECRUITERS',
          'WORKABLE',
          'WORKDAY',
          'CAREER_PAGE'
      )
      AND js.enabled = true;

    RETURN NULL;
END;
$$;
-- +goose StatementEnd

DROP INDEX IF EXISTS job_sources_company_direct_idx;
CREATE INDEX job_sources_company_direct_idx
    ON job_sources (company_id, source_type)
    WHERE source_type IN (
        'GREENHOUSE',
        'LEVER',
        'ASHBY',
        'SMARTRECRUITERS',
        'WORKABLE',
        'WORKDAY',
        'CAREER_PAGE'
    );

ALTER TABLE job_sources DROP CONSTRAINT job_sources_source_type_check;
ALTER TABLE job_sources ADD CONSTRAINT job_sources_source_type_check
    CHECK (source_type IN (
        'GREENHOUSE',
        'LEVER',
        'ASHBY',
        'ARBEITNOW',
        'SMARTRECRUITERS',
        'WORKABLE',
        'WORKDAY',
        'CAREER_PAGE',
        'BRIGHTDATA',
        'SERPAPI_GOOGLE_JOBS'
    ));
