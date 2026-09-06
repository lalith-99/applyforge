-- name: CreateCandidateProfileVersion :one
INSERT INTO candidate_profile_versions (
    user_id, version, target_roles, seniority, years_experience, core_skills, secondary_skills,
    transferable_skills, domains, architecture_strengths, leadership_signals, experience_evidence,
    summary, source_content_hash
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
RETURNING id, user_id, version, target_roles, seniority, years_experience, core_skills,
    secondary_skills, transferable_skills, domains, architecture_strengths, leadership_signals,
    experience_evidence, summary, source_content_hash, created_at;

-- name: GetLatestCandidateProfileVersion :one
SELECT id, user_id, version, target_roles, seniority, years_experience, core_skills,
    secondary_skills, transferable_skills, domains, architecture_strengths, leadership_signals,
    experience_evidence, summary, source_content_hash, created_at
FROM candidate_profile_versions
WHERE user_id = $1
ORDER BY version DESC
LIMIT 1;

-- name: UpdateCandidateProfileEmbedding :exec
UPDATE candidate_profile_versions SET embedding = $2, embedding_model = $3, embedded_at = now()
WHERE id = $1;
