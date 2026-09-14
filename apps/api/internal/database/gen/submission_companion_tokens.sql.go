package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type SubmissionCompanionToken struct {
	ID         pgtype.UUID        `json:"id"`
	UserID     pgtype.UUID        `json:"user_id"`
	IntentID   pgtype.UUID        `json:"intent_id"`
	TokenHash  string             `json:"token_hash"`
	ExpiresAt  pgtype.Timestamptz `json:"expires_at"`
	LastUsedAt pgtype.Timestamptz `json:"last_used_at"`
	RevokedAt  pgtype.Timestamptz `json:"revoked_at"`
	CreatedAt  pgtype.Timestamptz `json:"created_at"`
}

type RevokeSubmissionCompanionTokensForIntentParams struct {
	UserID   pgtype.UUID
	IntentID pgtype.UUID
}

const revokeSubmissionCompanionTokensForIntent = `
UPDATE submission_companion_tokens
SET revoked_at = COALESCE(revoked_at, now())
WHERE user_id = $1 AND intent_id = $2 AND revoked_at IS NULL
`

func (q *Queries) RevokeSubmissionCompanionTokensForIntent(ctx context.Context, arg RevokeSubmissionCompanionTokensForIntentParams) error {
	_, err := q.db.Exec(ctx, revokeSubmissionCompanionTokensForIntent, arg.UserID, arg.IntentID)
	return err
}

type CreateSubmissionCompanionTokenParams struct {
	UserID    pgtype.UUID
	IntentID  pgtype.UUID
	TokenHash string
	ExpiresAt pgtype.Timestamptz
}

const createSubmissionCompanionToken = `
INSERT INTO submission_companion_tokens (user_id, intent_id, token_hash, expires_at)
SELECT $1, i.id, $3, $4
FROM submission_intents i
WHERE i.id = $2
  AND i.user_id = $1
  AND i.status = 'PENDING'
RETURNING id, user_id, intent_id, token_hash, expires_at, last_used_at, revoked_at, created_at
`

func scanSubmissionCompanionToken(row interface{ Scan(...any) error }) (SubmissionCompanionToken, error) {
	var t SubmissionCompanionToken
	err := row.Scan(&t.ID, &t.UserID, &t.IntentID, &t.TokenHash, &t.ExpiresAt, &t.LastUsedAt, &t.RevokedAt, &t.CreatedAt)
	return t, err
}

func (q *Queries) CreateSubmissionCompanionToken(ctx context.Context, arg CreateSubmissionCompanionTokenParams) (SubmissionCompanionToken, error) {
	return scanSubmissionCompanionToken(q.db.QueryRow(ctx, createSubmissionCompanionToken, arg.UserID, arg.IntentID, arg.TokenHash, arg.ExpiresAt))
}

const getActiveSubmissionCompanionToken = `
UPDATE submission_companion_tokens
SET last_used_at = now()
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > now()
RETURNING id, user_id, intent_id, token_hash, expires_at, last_used_at, revoked_at, created_at
`

func (q *Queries) GetActiveSubmissionCompanionToken(ctx context.Context, tokenHash string) (SubmissionCompanionToken, error) {
	return scanSubmissionCompanionToken(q.db.QueryRow(ctx, getActiveSubmissionCompanionToken, tokenHash))
}

const revokeSubmissionCompanionToken = `
UPDATE submission_companion_tokens
SET revoked_at = COALESCE(revoked_at, now())
WHERE token_hash = $1
`

func (q *Queries) RevokeSubmissionCompanionToken(ctx context.Context, tokenHash string) error {
	_, err := q.db.Exec(ctx, revokeSubmissionCompanionToken, tokenHash)
	return err
}
