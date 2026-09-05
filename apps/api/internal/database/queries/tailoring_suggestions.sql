-- name: CreateTailoringSuggestion :one
INSERT INTO tailoring_suggestions (
    tailoring_run_id, section, original_text, suggested_text, requirements_addressed,
    skills_added, keywords_added, source, reason, confidence, risk_level
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListTailoringSuggestions :many
SELECT * FROM tailoring_suggestions WHERE tailoring_run_id = $1 ORDER BY created_at ASC;

-- name: GetTailoringSuggestion :one
SELECT * FROM tailoring_suggestions WHERE id = $1 AND tailoring_run_id = $2;

-- name: UpdateTailoringSuggestionStatus :one
UPDATE tailoring_suggestions
SET user_status = $3, edited_text = $4, updated_at = now()
WHERE id = $1 AND tailoring_run_id = $2
RETURNING *;

-- name: ApproveAllPendingSuggestions :exec
UPDATE tailoring_suggestions
SET user_status = 'APPROVED', updated_at = now()
WHERE tailoring_run_id = $1 AND user_status = 'PENDING';
