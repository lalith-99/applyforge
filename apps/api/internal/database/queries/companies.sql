-- name: UpsertCompany :one
INSERT INTO companies (name, normalized_name, domain)
VALUES ($1, $2, $3)
ON CONFLICT (normalized_name) DO UPDATE SET name = EXCLUDED.name
RETURNING *;

-- name: GetCompanyByNormalizedName :one
SELECT * FROM companies WHERE normalized_name = $1;

-- name: ListJobSources :many
SELECT * FROM job_sources WHERE enabled = true ORDER BY created_at ASC;

-- name: TouchJobSource :exec
UPDATE job_sources SET last_polled_at = now(), last_error = $2 WHERE id = $1;
