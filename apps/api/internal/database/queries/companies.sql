-- name: UpsertCompany :one
INSERT INTO companies (name, normalized_name, domain)
VALUES ($1, $2, $3)
ON CONFLICT (normalized_name) DO UPDATE SET name = EXCLUDED.name
RETURNING *;

-- name: GetCompanyByNormalizedName :one
SELECT * FROM companies WHERE normalized_name = $1;

-- name: CreateJobSource :one
INSERT INTO job_sources (source_type, company_id, board_token, enabled)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListJobSources :many
SELECT job_sources.*, companies.name AS company_name FROM job_sources
JOIN companies ON companies.id = job_sources.company_id
WHERE enabled = true
ORDER BY job_sources.created_at ASC;

-- name: GetJobSourceByID :one
SELECT job_sources.*, companies.name AS company_name FROM job_sources
JOIN companies ON companies.id = job_sources.company_id
WHERE job_sources.id = $1;

-- name: TouchJobSource :exec
UPDATE job_sources SET last_polled_at = now(), last_error = $2 WHERE id = $1;
