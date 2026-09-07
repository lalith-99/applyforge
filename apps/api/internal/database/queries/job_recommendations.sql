-- name: ReplaceJobRecommendations :exec
-- Replaces a user's whole recommendation set atomically (delete-then-insert
-- is simpler and cheap here since the set is small, N<=~50, and always
-- fully recomputed together rather than updated piecemeal).
DELETE FROM job_recommendations WHERE user_id = $1;

-- name: InsertJobRecommendation :exec
INSERT INTO job_recommendations (
    user_id, job_id, deterministic_score, ai_fit_score, ai_recommendation, ai_reason,
    final_score, candidate_profile_version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
);

-- name: ListJobRecommendations :many
-- Re-enforces country/software/status hard filters at read time (not just
-- at compute time) so a stale precomputed row - e.g. from before country_code
-- or role_classification existed - never leaks a non-US or non-software job
-- into a user's list while waiting for their next recompute cycle.
SELECT r.id, r.user_id, r.job_id, r.deterministic_score, r.ai_fit_score, r.ai_recommendation,
    r.ai_reason, r.final_score, r.candidate_profile_version, r.computed_at,
    j.title, j.company_name, j.location_text, j.remote_type, j.employment_type, j.apply_url
FROM job_recommendations r
JOIN jobs j ON j.id = r.job_id
WHERE r.user_id = $1
  AND j.status = 'ACTIVE' AND j.canonical_job_id IS NULL
  AND j.country_code = 'US' AND j.role_classification = 'IC_SOFTWARE'
  AND j.posted_at IS NOT NULL AND j.posted_at >= now() - INTERVAL '7 days'
ORDER BY r.final_score DESC
LIMIT $2;
