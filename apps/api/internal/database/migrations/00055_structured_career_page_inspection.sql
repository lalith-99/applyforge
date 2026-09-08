-- +goose Up
-- Inspect registry-only career sources (custom pages, iCIMS, Oracle and
-- legacy unresolved entries) and promote pages that expose structured
-- JobPosting data into a monitorable CAREER_PAGE source.

ALTER TABLE company_source_registry
    ADD COLUMN inspection_status TEXT NOT NULL DEFAULT 'PENDING',
    ADD COLUMN inspection_attempt_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN next_inspection_at TIMESTAMPTZ,
    ADD COLUMN last_inspection_at TIMESTAMPTZ,
    ADD COLUMN inspection_last_error TEXT,
    ADD CONSTRAINT company_source_registry_inspection_status_check
        CHECK (inspection_status IN ('PENDING', 'RESOLVED', 'FAILED', 'UNSUPPORTED'));

UPDATE company_source_registry
SET inspection_status = 'RESOLVED'
WHERE monitorable = true;

UPDATE company_source_registry
SET next_inspection_at = now()
WHERE monitorable = false
  AND source_type IN ('WORKDAY', 'ICIMS', 'ORACLE', 'CUSTOM');

CREATE INDEX company_source_registry_inspection_due_idx
    ON company_source_registry (
        inspection_status,
        next_inspection_at,
        company_id
    )
    WHERE monitorable = false;

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

-- Extend the sponsor-tier cadence trigger created with Workday support.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION sync_sponsor_direct_source_cadence()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        UPDATE job_sources
        SET enabled = false
        WHERE company_id = OLD.company_id
          AND source_type IN (
              'GREENHOUSE',
              'LEVER',
              'ASHBY',
              'SMARTRECRUITERS',
              'WORKABLE',
              'WORKDAY',
              'CAREER_PAGE'
          );
        RETURN OLD;
    END IF;

    UPDATE job_sources
    SET poll_interval_minutes = NEW.poll_interval_minutes
    WHERE company_id = NEW.company_id
      AND source_type IN (
          'GREENHOUSE',
          'LEVER',
          'ASHBY',
          'SMARTRECRUITERS',
          'WORKABLE',
          'WORKDAY',
          'CAREER_PAGE'
      );

    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose Down
DELETE FROM job_sources WHERE source_type = 'CAREER_PAGE';
DELETE FROM company_source_registry WHERE source_type = 'CAREER_PAGE';

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION sync_sponsor_direct_source_cadence()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        UPDATE job_sources
        SET enabled = false
        WHERE company_id = OLD.company_id
          AND source_type IN (
              'GREENHOUSE',
              'LEVER',
              'ASHBY',
              'SMARTRECRUITERS',
              'WORKABLE',
              'WORKDAY'
          );
        RETURN OLD;
    END IF;

    UPDATE job_sources
    SET poll_interval_minutes = NEW.poll_interval_minutes
    WHERE company_id = NEW.company_id
      AND source_type IN (
          'GREENHOUSE',
          'LEVER',
          'ASHBY',
          'SMARTRECRUITERS',
          'WORKABLE',
          'WORKDAY'
      );

    RETURN NEW;
END;
$$;
-- +goose StatementEnd

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
            'CUSTOM'
        )
    );

DROP INDEX IF EXISTS company_source_registry_inspection_due_idx;

ALTER TABLE company_source_registry
    DROP CONSTRAINT company_source_registry_inspection_status_check,
    DROP COLUMN inspection_last_error,
    DROP COLUMN last_inspection_at,
    DROP COLUMN next_inspection_at,
    DROP COLUMN inspection_attempt_count,
    DROP COLUMN inspection_status;
