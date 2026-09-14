-- +goose Up
-- Newly verified/direct sponsor sources should start yielding jobs immediately
-- instead of waiting for the next periodic source scheduler tick. Keep the
-- invariant at the database boundary because job_sources can be promoted by
-- several paths (inspection, broad-source ingestion, bootstrap/backfill).
--
-- This intentionally does not enqueue every historical never-polled source
-- during migration. Existing sources remain covered by the normal due-source
-- scheduler; avoiding a migration-time queue flood also keeps schema setup
-- free of operational side effects.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION enqueue_initial_job_source_sync()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.enabled = true AND NEW.last_polled_at IS NULL THEN
        INSERT INTO background_jobs (job_type, payload, max_attempts)
        VALUES (
            'sync_job_source',
            jsonb_build_object('job_source_id', NEW.id::text),
            3
        )
        ON CONFLICT DO NOTHING;
    END IF;

    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER job_sources_enqueue_initial_sync
AFTER INSERT OR UPDATE OF enabled, company_id, board_token
ON job_sources
FOR EACH ROW
EXECUTE FUNCTION enqueue_initial_job_source_sync();

-- +goose Down
DROP TRIGGER IF EXISTS job_sources_enqueue_initial_sync ON job_sources;
DROP FUNCTION IF EXISTS enqueue_initial_job_source_sync();
