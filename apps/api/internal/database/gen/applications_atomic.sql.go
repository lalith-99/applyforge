package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const changeApplicationStatusWithEvent = `-- name: ChangeApplicationStatusWithEvent :one
WITH current AS (
    SELECT id, status
    FROM applications
    WHERE id = $1 AND user_id = $2
    FOR UPDATE
), updated AS (
    UPDATE applications a
    SET status = $3,
        applied_at = CASE WHEN $3 = 'APPLIED' AND a.applied_at IS NULL THEN now() ELSE a.applied_at END,
        updated_at = now()
    FROM current c
    WHERE a.id = c.id
    RETURNING a.id, a.user_id, a.job_id, a.resume_version_id, a.status, a.match_score,
              a.notes, a.next_action, a.applied_at, a.created_at, a.updated_at,
              c.status AS from_status
), event_write AS (
    INSERT INTO application_events (application_id, event_type, from_status, to_status, notes)
    SELECT id, 'STATUS_CHANGE', from_status, status, $4
    FROM updated
    WHERE from_status <> status
    RETURNING application_id
)
SELECT u.id, u.user_id, u.job_id, u.resume_version_id, u.status, u.match_score,
       u.notes, u.next_action, u.applied_at, u.created_at, u.updated_at
FROM updated u
LEFT JOIN event_write e ON e.application_id = u.id
LIMIT 1
`

type ChangeApplicationStatusWithEventParams struct {
	ID     pgtype.UUID `json:"id"`
	UserID pgtype.UUID `json:"user_id"`
	Status string      `json:"status"`
	Notes  pgtype.Text `json:"notes"`
}

func (q *Queries) ChangeApplicationStatusWithEvent(ctx context.Context, arg ChangeApplicationStatusWithEventParams) (Application, error) {
	row := q.db.QueryRow(ctx, changeApplicationStatusWithEvent,
		arg.ID,
		arg.UserID,
		arg.Status,
		arg.Notes,
	)
	var i Application
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.JobID,
		&i.ResumeVersionID,
		&i.Status,
		&i.MatchScore,
		&i.Notes,
		&i.NextAction,
		&i.AppliedAt,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}
