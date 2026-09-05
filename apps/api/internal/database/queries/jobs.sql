-- name: UpsertJob :one
INSERT INTO jobs (
    source, external_id, company_id, company_name, title, normalized_title, seniority, description,
    country, state, city, location_text, remote_type, employment_type,
    salary_min, salary_max, salary_currency, apply_url, source_url, posted_at, content_hash
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21
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
    updated_at = now(),
    last_seen_at = now()
RETURNING *, (xmax = 0) AS inserted;

-- name: GetJobByID :one
SELECT * FROM jobs WHERE id = $1;

-- name: CountJobs :one
SELECT count(*) FROM jobs
WHERE status = 'ACTIVE'
  AND ($1::text = '' OR title ILIKE '%' || $1 || '%' OR company_name ILIKE '%' || $1 || '%')
  AND ($2::text = '' OR remote_type = $2)
  AND ($3::text = '' OR employment_type = $3)
  AND ($4::timestamptz IS NULL OR posted_at >= $4 OR (posted_at IS NULL AND first_seen_at >= $4));

-- name: ListJobs :many
SELECT * FROM jobs
WHERE status = 'ACTIVE'
  AND ($1::text = '' OR title ILIKE '%' || $1 || '%' OR company_name ILIKE '%' || $1 || '%')
  AND ($2::text = '' OR remote_type = $2)
  AND ($3::text = '' OR employment_type = $3)
  AND ($4::timestamptz IS NULL OR posted_at >= $4 OR (posted_at IS NULL AND first_seen_at >= $4))
ORDER BY
  CASE WHEN $5::text = 'newest' THEN coalesce(posted_at, first_seen_at) END DESC,
  CASE WHEN $5::text = 'salary' THEN coalesce(salary_max, salary_min, 0) END DESC,
  first_seen_at DESC
LIMIT $6 OFFSET $7;
