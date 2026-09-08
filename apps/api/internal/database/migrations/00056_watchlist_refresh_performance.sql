-- +goose Up
-- A 10k-company watchlist refresh used to fire one source-cadence UPDATE per
-- inserted/updated/deleted company. Replace that row trigger with transition
-- table statement triggers so each watchlist statement updates job_sources in
-- one set-based operation.

DROP TRIGGER IF EXISTS company_sponsor_watchlist_direct_source_cadence
    ON company_sponsor_watchlist;

CREATE INDEX IF NOT EXISTS job_sources_company_direct_idx
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

CREATE TRIGGER company_sponsor_watchlist_direct_source_cadence_insert
AFTER INSERT ON company_sponsor_watchlist
REFERENCING NEW TABLE AS sponsor_rows_new
FOR EACH STATEMENT
EXECUTE FUNCTION sync_sponsor_direct_source_cadence_insert_stmt();

CREATE TRIGGER company_sponsor_watchlist_direct_source_cadence_update
AFTER UPDATE ON company_sponsor_watchlist
REFERENCING OLD TABLE AS sponsor_rows_old NEW TABLE AS sponsor_rows_new
FOR EACH STATEMENT
EXECUTE FUNCTION sync_sponsor_direct_source_cadence_update_stmt();

CREATE TRIGGER company_sponsor_watchlist_direct_source_cadence_delete
AFTER DELETE ON company_sponsor_watchlist
REFERENCING OLD TABLE AS sponsor_rows_old
FOR EACH STATEMENT
EXECUTE FUNCTION sync_sponsor_direct_source_cadence_delete_stmt();

-- +goose Down
DROP TRIGGER IF EXISTS company_sponsor_watchlist_direct_source_cadence_insert
    ON company_sponsor_watchlist;
DROP TRIGGER IF EXISTS company_sponsor_watchlist_direct_source_cadence_update
    ON company_sponsor_watchlist;
DROP TRIGGER IF EXISTS company_sponsor_watchlist_direct_source_cadence_delete
    ON company_sponsor_watchlist;

DROP FUNCTION IF EXISTS sync_sponsor_direct_source_cadence_insert_stmt();
DROP FUNCTION IF EXISTS sync_sponsor_direct_source_cadence_update_stmt();
DROP FUNCTION IF EXISTS sync_sponsor_direct_source_cadence_delete_stmt();

DROP INDEX IF EXISTS job_sources_company_direct_idx;

CREATE TRIGGER company_sponsor_watchlist_direct_source_cadence
AFTER INSERT OR UPDATE OF poll_interval_minutes OR DELETE
ON company_sponsor_watchlist
FOR EACH ROW
EXECUTE FUNCTION sync_sponsor_direct_source_cadence();
