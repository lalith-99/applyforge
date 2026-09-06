-- name: UpsertJob :one
INSERT INTO jobs (
    source, external_id, company_id, company_name, title, normalized_title, seniority, description,
    country, state, city, location_text, remote_type, employment_type,
    salary_min, salary_max, salary_currency, apply_url, source_url, posted_at, content_hash, fingerprint
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
)
ON CONFLICT (source, external_id) DO UPDATE SET
    company_id = EXCLUDED.company_id,
    company_name = EXCLUDED.company_name,
    title = EXCLUDED.title,
    normalized_title = EXCLUDED.normalized_title,
    seniority = EXCLUDED.seniority,
    description = EXCLUDED.description,
    country = EXCLUDED.country,
    state = EXCLUDED.state,
    city = EXCLUDED.city,
    location_text = EXCLUDED.location_text,
    remote_type = EXCLUDED.remote_type,
    employment_type = EXCLUDED.employment_type,
    salary_min = EXCLUDED.salary_min,
    salary_max = EXCLUDED.salary_max,
    salary_currency = EXCLUDED.salary_currency,
    apply_url = EXCLUDED.apply_url,
    source_url = EXCLUDED.source_url,
    posted_at = EXCLUDED.posted_at,
    content_hash = EXCLUDED.content_hash,
    fingerprint = EXCLUDED.fingerprint,
    status = 'ACTIVE',
    updated_at = now(),
    last_seen_at = now()
RETURNING *, (xmax = 0) AS inserted;

-- name: FindCanonicalByFingerprint :one
-- Finds an existing, still-canonical job with the same fingerprint from a
-- DIFFERENT source row (cross-source dedupe target). Excludes jobID itself
-- so a job never becomes its own canonical.
SELECT * FROM jobs
WHERE fingerprint = $1 AND fingerprint != '' AND canonical_job_id IS NULL AND id != $2
ORDER BY first_seen_at ASC
LIMIT 1;

-- name: SetCanonicalJobID :exec
UPDATE jobs SET canonical_job_id = $2, updated_at = now() WHERE id = $1;

-- name: CloseStaleJobs :execrows
-- Marks jobs CLOSED when a source poll completed without re-seeing them
-- (last_seen_at predates the poll's start). Only meaningful for sources that
-- fetch their FULL current listing every poll (Greenhouse/Lever/Ashby); an
-- aggregator with a page cap (Arbeitnow) never calls this - "not seen this
-- poll" would just mean "pushed past the page cap", not "actually closed".
UPDATE jobs SET status = 'CLOSED', updated_at = now()
WHERE source = $1 AND company_id = $2 AND status = 'ACTIVE' AND last_seen_at < $3;

-- name: GetJobByID :one
SELECT * FROM jobs WHERE id = $1;

-- name: CountJobs :one
SELECT count(*) FROM jobs
WHERE status = 'ACTIVE' AND canonical_job_id IS NULL
  AND ($1::text = '' OR title ILIKE '%' || $1 || '%' OR company_name ILIKE '%' || $1 || '%')
  AND ($2::text = '' OR remote_type = $2)
  AND ($3::text = '' OR employment_type = $3)
  AND ($4::timestamptz IS NULL OR posted_at >= $4 OR (posted_at IS NULL AND first_seen_at >= $4))
  AND ($5::text = '' OR location_text ILIKE '%' || $5 || '%' OR city ILIKE '%' || $5 || '%' OR state ILIKE '%' || $5 || '%' OR country ILIKE '%' || $5 || '%');

-- name: ListJobs :many
SELECT * FROM jobs
WHERE status = 'ACTIVE' AND canonical_job_id IS NULL
  AND ($1::text = '' OR title ILIKE '%' || $1 || '%' OR company_name ILIKE '%' || $1 || '%')
  AND ($2::text = '' OR remote_type = $2)
  AND ($3::text = '' OR employment_type = $3)
  AND ($4::timestamptz IS NULL OR posted_at >= $4 OR (posted_at IS NULL AND first_seen_at >= $4))
  AND ($5::text = '' OR location_text ILIKE '%' || $5 || '%' OR city ILIKE '%' || $5 || '%' OR state ILIKE '%' || $5 || '%' OR country ILIKE '%' || $5 || '%')
ORDER BY
  CASE WHEN $6::text = 'newest' THEN coalesce(posted_at, first_seen_at) END DESC,
  CASE WHEN $6::text = 'salary' THEN coalesce(salary_max, salary_min, 0) END DESC,
  first_seen_at DESC
LIMIT $7 OFFSET $8;
