-- name: EnqueueJob :one
INSERT INTO background_jobs (job_type, payload, max_attempts)
VALUES ($1, $2, $3)
RETURNING *;

-- name: FindJobByTypeAndPayload :one
SELECT * FROM background_jobs
WHERE job_type = $1 AND payload @> sqlc.arg(match_payload)::jsonb
ORDER BY created_at DESC
LIMIT 1;

-- name: ClaimNextJob :one
-- Reclaim leases abandoned by a crashed/restarted worker before choosing the
-- next PENDING job. A two-hour lease is intentionally conservative for direct
-- ATS polls and AI work. Exhausted jobs become DEAD_LETTER; retryable jobs are
-- returned to PENDING and can be claimed on the next poll iteration.
--
-- Interactive, user-triggered job types (parse_resume, build_candidate_profile,
-- compute_recommendations, process_tailoring_run) are claimed ahead of bulk
-- background ingestion (enrich_job, embed_job, sync_job_source), so a large
-- ingestion backlog never stalls a user actively waiting on a result.
WITH expired_leases AS (
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
      AND (locked_at IS NULL OR locked_at < now() - INTERVAL '2 hours')
)
UPDATE background_jobs
SET status = 'RUNNING', attempts = attempts + 1, locked_at = now(), locked_by = $1
WHERE id = (
    SELECT id FROM background_jobs
    WHERE status = 'PENDING' AND available_at <= now()
    ORDER BY
        CASE WHEN job_type IN ('parse_resume', 'build_candidate_profile', 'compute_recommendations', 'process_tailoring_run') THEN 0 ELSE 1 END,
        available_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: CompleteJob :exec
UPDATE background_jobs SET status = 'COMPLETED', completed_at = now() WHERE id = $1;

-- name: FailJob :exec
UPDATE background_jobs
SET status = CASE WHEN attempts >= max_attempts THEN 'DEAD_LETTER' ELSE 'PENDING' END,
    last_error = $2,
    available_at = now() + ($3::int * interval '1 second')
WHERE id = $1;
