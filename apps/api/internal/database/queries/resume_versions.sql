-- name: CreateResumeVersion :one
INSERT INTO resume_versions (
    user_id, base_resume_id, job_id, tailoring_run_id, version_number, content_json,
    match_score, alignment_score, tailoring_mode
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: SetResumeVersionDocuments :one
UPDATE resume_versions SET pdf_storage_key = $2, docx_storage_key = $3 WHERE id = $1
RETURNING *;

-- name: GetNextResumeVersionNumber :one
SELECT COALESCE(MAX(version_number), 0) + 1 FROM resume_versions WHERE base_resume_id = $1;

-- name: GetResumeVersionForUser :one
SELECT * FROM resume_versions WHERE id = $1 AND user_id = $2;

-- name: ListResumeVersionsForResume :many
SELECT * FROM resume_versions WHERE base_resume_id = $1 ORDER BY version_number DESC;
