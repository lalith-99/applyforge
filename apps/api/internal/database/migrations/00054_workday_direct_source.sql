-- +goose Up
-- Workday public career sites expose a tenant/site scoped CXS JSON feed.
-- Enable it as an authoritative direct source once the exact tenant/site URL
-- has been discovered from an observed job/career URL.

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

-- Keep newly-supported Workday sources aligned with sponsor-watchlist tier
-- cadence even after a later DOL watchlist refresh.
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

CREATE TRIGGER company_sponsor_watchlist_direct_source_cadence
AFTER INSERT OR UPDATE OF poll_interval_minutes OR DELETE
ON company_sponsor_watchlist
FOR EACH ROW
EXECUTE FUNCTION sync_sponsor_direct_source_cadence();

-- +goose Down
DROP TRIGGER IF EXISTS company_sponsor_watchlist_direct_source_cadence
    ON company_sponsor_watchlist;
DROP FUNCTION IF EXISTS sync_sponsor_direct_source_cadence();

DELETE FROM job_sources WHERE source_type = 'WORKDAY';

ALTER TABLE job_sources DROP CONSTRAINT job_sources_source_type_check;
ALTER TABLE job_sources ADD CONSTRAINT job_sources_source_type_check
    CHECK (source_type IN (
        'GREENHOUSE',
        'LEVER',
        'ASHBY',
        'ARBEITNOW',
        'SMARTRECRUITERS',
        'WORKABLE',
        'BRIGHTDATA',
        'SERPAPI_GOOGLE_JOBS'
    ));
