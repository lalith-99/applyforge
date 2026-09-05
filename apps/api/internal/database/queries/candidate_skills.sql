-- name: UpsertCandidateSkill :one
INSERT INTO candidate_skills (user_id, normalized_name, display_name, category, proficiency, source, status)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (user_id, normalized_name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    category = EXCLUDED.category,
    proficiency = COALESCE(EXCLUDED.proficiency, candidate_skills.proficiency),
    source = EXCLUDED.source,
    status = EXCLUDED.status,
    updated_at = now()
RETURNING *;

-- name: ListCandidateSkillsForUser :many
SELECT * FROM candidate_skills WHERE user_id = $1 ORDER BY display_name ASC;

-- name: UpdateCandidateSkillStatus :one
UPDATE candidate_skills SET status = $3, updated_at = now()
WHERE user_id = $1 AND normalized_name = $2
RETURNING *;
