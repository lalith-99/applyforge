package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const listLatestSubmissionIntentsForUser = `
SELECT DISTINCT ON (application_id)
       id, user_id, application_id, package_id, approval_id, package_hash,
       idempotency_key, status, lease_generation, lease_owner, lease_expires_at,
       attempt_count, last_error, created_at, updated_at, completed_at
FROM submission_intents
WHERE user_id = $1
ORDER BY application_id, created_at DESC, id DESC
`

func (q *Queries) ListLatestSubmissionIntentsForUser(ctx context.Context, userID pgtype.UUID) ([]SubmissionIntent, error) {
	rows, err := q.db.Query(ctx, listLatestSubmissionIntentsForUser, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SubmissionIntent
	for rows.Next() {
		intent, err := scanSubmissionIntent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, intent)
	}
	return result, rows.Err()
}
