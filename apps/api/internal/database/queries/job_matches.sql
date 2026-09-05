-- name: UpsertJobMatch :one
INSERT INTO job_matches (
    job_id, user_id, total_score, grade, component_scores, matched_skills, transferable_skills,
    missing_required_skills, missing_preferred_skills, positive_evidence, concerns, explanation,
    opportunity_score, current_profile_match, target_profile_match, suggested_target_additions,
    eligible, hard_failures, warnings
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
)
ON CONFLICT (job_id, user_id) DO UPDATE SET
    total_score = EXCLUDED.total_score,
    grade = EXCLUDED.grade,
    component_scores = EXCLUDED.component_scores,
    matched_skills = EXCLUDED.matched_skills,
    transferable_skills = EXCLUDED.transferable_skills,
    missing_required_skills = EXCLUDED.missing_required_skills,
    missing_preferred_skills = EXCLUDED.missing_preferred_skills,
    positive_evidence = EXCLUDED.positive_evidence,
    concerns = EXCLUDED.concerns,
    explanation = EXCLUDED.explanation,
    opportunity_score = EXCLUDED.opportunity_score,
    current_profile_match = EXCLUDED.current_profile_match,
    target_profile_match = EXCLUDED.target_profile_match,
    suggested_target_additions = EXCLUDED.suggested_target_additions,
    eligible = EXCLUDED.eligible,
    hard_failures = EXCLUDED.hard_failures,
    warnings = EXCLUDED.warnings,
    computed_at = now()
RETURNING *;

-- name: GetJobMatch :one
SELECT * FROM job_matches WHERE job_id = $1 AND user_id = $2;

-- name: ListJobMatchesForUser :many
SELECT * FROM job_matches WHERE user_id = $1;
