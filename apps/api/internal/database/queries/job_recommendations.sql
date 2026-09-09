-- name: ReplaceJobRecommendations :exec
-- Replaces a user's whole recommendation set atomically (delete-then-insert
-- is simpler and cheap here since the set is small, N<=~50, and always
-- fully recomputed together rather than updated piecemeal).
DELETE FROM job_recommendations WHERE user_id = $1;

-- name: InsertJobRecommendation :exec
INSERT INTO job_recommendations (
    user_id, job_id, deterministic_score, ai_fit_score, ai_recommendation, ai_reason,
    final_score, candidate_profile_version, immigration_status, immigration_confidence,
    immigration_evidence, immigration_priority_score
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
);

-- name: ListJobRecommendations :many
-- Re-enforces country/software/status hard filters at read time (not just
-- at compute time) so a stale precomputed row - e.g. from before country_code
-- or role_classification existed - never leaks a non-US or non-software job
-- into a user's list while waiting for their next recompute cycle.
SELECT r.id, r.user_id, r.job_id, r.deterministic_score, r.ai_fit_score, r.ai_recommendation,
    r.ai_reason, r.final_score, r.candidate_profile_version, r.immigration_status,
    r.immigration_confidence, r.immigration_evidence, r.immigration_priority_score, r.computed_at,
    j.title, j.company_name, j.location_text, j.remote_type, j.employment_type, j.apply_url
FROM job_recommendations r
JOIN jobs j ON j.id = r.job_id
LEFT JOIN job_preferences p ON p.user_id = r.user_id
WHERE r.user_id = $1
  AND j.status = 'ACTIVE' AND j.canonical_job_id IS NULL
  AND j.country_code = 'US' AND j.role_classification = 'IC_SOFTWARE'
  AND j.posted_at IS NOT NULL AND j.posted_at >= now() - INTERVAL '24 hours'
  AND (
      j.explicit_sponsorship_denied = false
      OR NOT (
          coalesce(p.requires_h1b_transfer, false)
          OR coalesce(p.requires_new_h1b_cap_sponsorship, false)
          OR coalesce(p.requires_future_employment_sponsorship, false)
          OR regexp_replace(lower(coalesce(p.immigration_status, '')), '[- _]', '', 'g') LIKE '%h1b%'
          OR regexp_replace(lower(coalesce(p.work_authorization, '')), '[- _]', '', 'g') LIKE '%h1b%'
      )
  )
ORDER BY r.final_score DESC, j.posted_at DESC
LIMIT $2;
