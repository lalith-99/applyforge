-- +goose Up
-- iCIMS became a verified direct connector after the structured source
-- registry was introduced. Keep the database source-type constraint and
-- sponsor-tier cadence trigger aligned with the Go connector set.

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
              'ICIMS',
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
          'ICIMS',
          'CAREER_PAGE'
      );

    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- Remove rows that would violate the restored pre-iCIMS constraint before
-- narrowing it. Registry candidates are intentionally preserved so upgrading
-- again can re-promote them without rediscovery.
DELETE FROM job_sources WHERE source_type = 'ICIMS';

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
