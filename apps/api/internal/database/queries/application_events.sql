-- name: CreateApplicationEvent :one
INSERT INTO application_events (application_id, event_type, from_status, to_status, notes)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListApplicationEvents :many
SELECT * FROM application_events WHERE application_id = $1 ORDER BY created_at ASC;
