-- name: CreateSubmissionCompanionToken :one
INSERT INTO submission_companion_tokens (user_id, intent_id, token_hash, expires_at)
SELECT $1, i.id, $3, $4
FROM submission_intents i
WHERE i.id = $2
  AND i.user_id = $1
  AND i.status = 'PENDING'
RETURNING *;

-- name: GetActiveSubmissionCompanionToken :one
UPDATE submission_companion_tokens
SET last_used_at = now()
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > now()
RETURNING *;

-- name: RevokeSubmissionCompanionToken :exec
UPDATE submission_companion_tokens
SET revoked_at = COALESCE(revoked_at, now())
WHERE token_hash = $1;
