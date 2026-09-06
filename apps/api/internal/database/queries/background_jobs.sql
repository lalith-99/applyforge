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
UPDATE background_jobs
SET status = 'RUNNING', attempts = attempts + 1, locked_at = now(), locked_by = $1
WHERE id = (
    SELECT id FROM background_jobs
    WHERE status = 'PENDING' AND available_at <= now()
    ORDER BY available_at ASC
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
