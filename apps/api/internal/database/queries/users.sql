-- name: CreateUserWithPassword :one
INSERT INTO users (email, password_hash)
VALUES ($1, $2)
RETURNING *;

-- name: CreateUserWithGoogle :one
INSERT INTO users (email, google_id, email_verified_at)
VALUES ($1, $2, now())
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE lower(email) = lower($1);

-- name: GetUserByGoogleID :one
SELECT * FROM users WHERE google_id = $1;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: LinkGoogleAccount :one
UPDATE users
SET google_id = $2, email_verified_at = COALESCE(email_verified_at, now()), updated_at = now()
WHERE id = $1
RETURNING *;
