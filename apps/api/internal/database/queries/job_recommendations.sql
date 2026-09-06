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
SELECT r.id, r.user_id, r.job_id, r.deterministic_score, r.ai_fit_score, r.ai_recommendation,
    r.ai_reason, r.final_score, r.candidate_profile_version, r.computed_at,
    j.title, j.company_name, j.location_text, j.remote_type, j.employment_type, j.apply_url
FROM job_recommendations r
JOIN jobs j ON j.id = r.job_id
WHERE r.user_id = $1
ORDER BY r.final_score DESC
LIMIT $2;
