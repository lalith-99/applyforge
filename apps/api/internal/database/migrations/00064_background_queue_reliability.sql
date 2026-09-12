-- +goose Up
-- Recover work abandoned by a previous API/container process. A two-hour
-- lease is intentionally conservative: direct ATS polls and AI work should
-- normally finish much sooner, while legitimate long-running work is not
-- reclaimed aggressively.
UPDATE background_jobs
SET status = CASE
        WHEN attempts >= max_attempts THEN 'DEAD_LETTER'
        ELSE 'PENDING'
    END,
    available_at = now(),
    locked_at = NULL,
    locked_by = NULL,
    last_error = 'background job lease expired after 2 hours'
WHERE status = 'RUNNING'
  AND (locked_at IS NULL OR locked_at < now() - INTERVAL '2 hours');

-- A disabled or deleted source must not keep occupying the sync queue. Only
-- pending work is removed here; a genuinely active RUNNING poll is allowed to
-- finish and will observe the source's latest enabled state in its handler.
DELETE FROM background_jobs bj
WHERE bj.job_type = 'sync_job_source'
  AND bj.status = 'PENDING'
  AND NOT EXISTS (
      SELECT 1
      FROM job_sources js
      WHERE js.id::text = bj.payload->>'job_source_id'
        AND js.enabled = true
  );

-- Historical scheduler runs could enqueue the same source repeatedly while an
-- earlier poll was still pending/running. Keep one active row per source,
-- preferring genuinely RUNNING work and then the least-attempted/oldest row.
WITH ranked AS (
    SELECT
        id,
        row_number() OVER (
            PARTITION BY payload->>'job_source_id'
            ORDER BY
                CASE WHEN status = 'RUNNING' THEN 0 ELSE 1 END,
                attempts ASC,
                available_at ASC,
                created_at ASC,
                id ASC
        ) AS rn
    FROM background_jobs
    WHERE job_type = 'sync_job_source'
      AND status IN ('PENDING', 'RUNNING')
      AND COALESCE(payload->>'job_source_id', '') <> ''
)
DELETE FROM background_jobs bj
USING ranked r
WHERE bj.id = r.id
  AND r.rn > 1;

-- Make the invariant race-safe at the database boundary. EnqueueSyncTasks also
-- debounces before insert, but this index protects concurrent scheduler/admin
-- triggers from creating two active polls for the same source.
CREATE UNIQUE INDEX background_jobs_active_sync_source_idx
    ON background_jobs ((payload->>'job_source_id'))
    WHERE job_type = 'sync_job_source'
      AND status IN ('PENDING', 'RUNNING')
      AND COALESCE(payload->>'job_source_id', '') <> '';

-- Claim-time stale-lease recovery only scans RUNNING work; keep that bounded.
CREATE INDEX background_jobs_running_lock_idx
    ON background_jobs (locked_at)
    WHERE status = 'RUNNING';

-- +goose Down
DROP INDEX IF EXISTS background_jobs_running_lock_idx;
DROP INDEX IF EXISTS background_jobs_active_sync_source_idx;
-- Queue cleanup and stale-lease recovery are data repairs and are intentionally
-- not reversed.
