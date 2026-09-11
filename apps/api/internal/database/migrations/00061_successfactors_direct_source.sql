-- +goose Up
-- Promote verified SAP SuccessFactors public feeds into the same direct-source
-- control plane as the other employer ATS connectors.

ALTER TABLE company_source_registry
    DROP CONSTRAINT company_source_registry_type_check;
ALTER TABLE company_source_registry
    ADD CONSTRAINT company_source_registry_type_check CHECK (
        source_type IN (
            'GREENHOUSE',
            'LEVER',
            'ASHBY',
            'SMARTRECRUITERS',
            'WORKABLE',
            'WORKDAY',
            'ICIMS',
            'SUCCESSFACTORS',
            'ORACLE',
            'CUSTOM',
            'CAREER_PAGE'
        )
    );

-- Older free-source bootstrap runs stored SuccessFactors inventory entries as
-- CUSTOM with a SUCCESSFACTORS|... board-token prefix. Keep those rows in place
-- so the inspection worker can verify provider identity from source_url and
-- promote a clean SUCCESSFACTORS row, but make them immediately eligible for
-- reinspection under the new connector.
UPDATE company_source_registry
SET inspection_status = 'PENDING',
    next_inspection_at = now(),
    inspection_last_error = NULL
WHERE source_type = 'CUSTOM'
  AND upper(board_token) LIKE 'SUCCESSFACTORS|%'
  AND monitorable = false;

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
        'SUCCESSFACTORS',
        'CAREER_PAGE',
        'BRIGHTDATA',
        'SERPAPI_GOOGLE_JOBS'
    ));

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
          'SUCCESSFACTORS',
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
          'SUCCESSFACTORS',
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
          'SUCCESSFACTORS',
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
        'SUCCESSFACTORS',
        'CAREER_PAGE'
    );

-- +goose Down
DELETE FROM job_sources WHERE source_type = 'SUCCESSFACTORS';
DELETE FROM company_source_registry WHERE source_type = 'SUCCESSFACTORS';

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

ALTER TABLE company_source_registry
    DROP CONSTRAINT company_source_registry_type_check;
ALTER TABLE company_source_registry
    ADD CONSTRAINT company_source_registry_type_check CHECK (
        source_type IN (
            'GREENHOUSE',
            'LEVER',
            'ASHBY',
            'SMARTRECRUITERS',
            'WORKABLE',
            'WORKDAY',
            'ICIMS',
            'ORACLE',
            'CUSTOM',
            'CAREER_PAGE'
        )
    );
