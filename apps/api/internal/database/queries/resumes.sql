-- name: CreateResume :one
INSERT INTO resumes (user_id, original_filename, mime_type, size_bytes, storage_key)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetResumeByID :one
SELECT * FROM resumes WHERE id = $1;

-- name: GetResumeForUser :one
SELECT * FROM resumes WHERE id = $1 AND user_id = $2;

-- name: ListResumesForUser :many
SELECT * FROM resumes WHERE user_id = $1 ORDER BY created_at DESC;

-- name: DeleteResume :exec
DELETE FROM resumes WHERE id = $1 AND user_id = $2;

-- name: MarkResumeParsing :exec
UPDATE resumes SET status = 'PARSING', updated_at = now() WHERE id = $1;

-- name: MarkResumeParsed :exec
UPDATE resumes
SET status = 'PARSED', raw_text = $2, parsed_profile = $3, parsed_at = now(), parse_error = NULL, updated_at = now()
WHERE id = $1;

-- name: MarkResumeFailed :exec
UPDATE resumes SET status = 'FAILED', parse_error = $2, updated_at = now() WHERE id = $1;

-- name: SetResumeStorageKey :exec
UPDATE resumes SET storage_key = $2, updated_at = now() WHERE id = $1;
