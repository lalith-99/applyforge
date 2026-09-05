-- name: CreateApplication :one
INSERT INTO applications (user_id, job_id, resume_version_id, status, match_score)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, job_id) DO UPDATE SET
    resume_version_id = COALESCE(EXCLUDED.resume_version_id, applications.resume_version_id),
    updated_at = now()
RETURNING *;

-- name: GetApplicationForUser :one
SELECT * FROM applications WHERE id = $1 AND user_id = $2;

-- name: ListApplicationsForUser :many
SELECT * FROM applications WHERE user_id = $1 ORDER BY updated_at DESC;

-- name: UpdateApplicationStatus :one
UPDATE applications
SET status = $3, applied_at = CASE WHEN $3 = 'APPLIED' AND applied_at IS NULL THEN now() ELSE applied_at END,
    updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: UpdateApplicationNotes :one
UPDATE applications SET notes = $3, next_action = $4, updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: CountApplicationsByStatusForUser :many
SELECT status, count(*) AS total FROM applications WHERE user_id = $1 GROUP BY status;

-- name: ListApplicationsWithJobForUser :many
SELECT a.*, j.company_name, j.title, j.normalized_title, j.source, j.first_seen_at, j.posted_at
FROM applications a
JOIN jobs j ON j.id = a.job_id
WHERE a.user_id = $1
ORDER BY a.updated_at DESC;

-- name: CountTailoringRunsForUser :one
SELECT count(*) FROM tailoring_runs WHERE user_id = $1;

-- name: CountHighMatchesForUser :one
SELECT count(*) FROM job_matches WHERE user_id = $1 AND total_score >= 90;

-- name: CountJobsDiscovered :one
SELECT count(*) FROM jobs WHERE status = 'ACTIVE';

-- name: CountApplicationEventsByToStatusForUser :many
SELECT ae.to_status, count(DISTINCT ae.application_id) AS total
FROM application_events ae
JOIN applications a ON a.id = ae.application_id
WHERE a.user_id = $1
GROUP BY ae.to_status;
