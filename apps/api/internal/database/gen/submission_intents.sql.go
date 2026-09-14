package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type SubmissionIntent struct {
	ID              pgtype.UUID        `json:"id"`
	UserID          pgtype.UUID        `json:"user_id"`
	ApplicationID   pgtype.UUID        `json:"application_id"`
	PackageID       pgtype.UUID        `json:"package_id"`
	ApprovalID      pgtype.UUID        `json:"approval_id"`
	PackageHash     string             `json:"package_hash"`
	IdempotencyKey  string             `json:"idempotency_key"`
	Status          string             `json:"status"`
	LeaseGeneration int64              `json:"lease_generation"`
	LeaseOwner      pgtype.Text        `json:"lease_owner"`
	LeaseExpiresAt  pgtype.Timestamptz `json:"lease_expires_at"`
	AttemptCount    int32              `json:"attempt_count"`
	LastError       pgtype.Text        `json:"last_error"`
	CreatedAt       pgtype.Timestamptz `json:"created_at"`
	UpdatedAt       pgtype.Timestamptz `json:"updated_at"`
	CompletedAt     pgtype.Timestamptz `json:"completed_at"`
}

func scanSubmissionIntent(row interface{ Scan(...any) error }) (SubmissionIntent, error) {
	var i SubmissionIntent
	err := row.Scan(
		&i.ID, &i.UserID, &i.ApplicationID, &i.PackageID, &i.ApprovalID,
		&i.PackageHash, &i.IdempotencyKey, &i.Status, &i.LeaseGeneration,
		&i.LeaseOwner, &i.LeaseExpiresAt, &i.AttemptCount, &i.LastError,
		&i.CreatedAt, &i.UpdatedAt, &i.CompletedAt,
	)
	return i, err
}

type CreateSubmissionIntentParams struct {
	PackageID      pgtype.UUID
	UserID         pgtype.UUID
	IdempotencyKey string
}

const createSubmissionIntent = `
INSERT INTO submission_intents (
    user_id, application_id, package_id, approval_id, package_hash, idempotency_key
)
SELECT p.user_id, p.application_id, p.id, a.id, p.package_hash, $3
FROM application_packages p
JOIN application_approvals a
  ON a.package_id = p.id
 AND a.user_id = p.user_id
 AND a.package_hash = p.package_hash
WHERE p.id = $1
  AND p.user_id = $2
  AND a.action_scope = 'SUBMIT_ONCE'
  AND a.revoked_at IS NULL
  AND a.expires_at > now()
ORDER BY a.approved_at DESC
LIMIT 1
ON CONFLICT (package_id) DO UPDATE
SET idempotency_key = submission_intents.idempotency_key
RETURNING id, user_id, application_id, package_id, approval_id, package_hash,
          idempotency_key, status, lease_generation, lease_owner, lease_expires_at,
          attempt_count, last_error, created_at, updated_at, completed_at
`

func (q *Queries) CreateSubmissionIntent(ctx context.Context, arg CreateSubmissionIntentParams) (SubmissionIntent, error) {
	return scanSubmissionIntent(q.db.QueryRow(ctx, createSubmissionIntent, arg.PackageID, arg.UserID, arg.IdempotencyKey))
}

type GetSubmissionIntentForUserParams struct {
	ID     pgtype.UUID
	UserID pgtype.UUID
}

const getSubmissionIntentForUser = `
SELECT id, user_id, application_id, package_id, approval_id, package_hash,
       idempotency_key, status, lease_generation, lease_owner, lease_expires_at,
       attempt_count, last_error, created_at, updated_at, completed_at
FROM submission_intents
WHERE id = $1 AND user_id = $2
`

func (q *Queries) GetSubmissionIntentForUser(ctx context.Context, arg GetSubmissionIntentForUserParams) (SubmissionIntent, error) {
	return scanSubmissionIntent(q.db.QueryRow(ctx, getSubmissionIntentForUser, arg.ID, arg.UserID))
}

const claimNextSubmissionIntent = `
WITH expired_submitting AS (
    UPDATE submission_intents
    SET status = 'UNCERTAIN',
        last_error = 'submission lease expired after irreversible phase began',
        lease_owner = NULL,
        lease_expires_at = NULL,
        updated_at = now()
    WHERE status = 'SUBMITTING'
      AND lease_expires_at < now()
), expired_claims AS (
    UPDATE submission_intents
    SET status = 'PENDING',
        lease_owner = NULL,
        lease_expires_at = NULL,
        updated_at = now()
    WHERE status = 'CLAIMED'
      AND lease_expires_at < now()
), candidate AS (
    SELECT id
    FROM submission_intents
    WHERE status = 'PENDING'
    ORDER BY created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
), claimed AS (
    UPDATE submission_intents i
    SET status = 'CLAIMED',
        lease_generation = i.lease_generation + 1,
        lease_owner = $1,
        lease_expires_at = now() + interval '5 minutes',
        attempt_count = i.attempt_count + 1,
        updated_at = now()
    FROM candidate c
    WHERE i.id = c.id
    RETURNING i.id, i.user_id, i.application_id, i.package_id, i.approval_id, i.package_hash,
              i.idempotency_key, i.status, i.lease_generation, i.lease_owner, i.lease_expires_at,
              i.attempt_count, i.last_error, i.created_at, i.updated_at, i.completed_at
), attempt_write AS (
    INSERT INTO submission_attempts (intent_id, lease_generation, worker_id, state)
    SELECT id, lease_generation, lease_owner, 'CLAIMED'
    FROM claimed
    RETURNING intent_id
)
SELECT c.*
FROM claimed c
JOIN attempt_write a ON a.intent_id = c.id
`

func (q *Queries) ClaimNextSubmissionIntent(ctx context.Context, workerID string) (SubmissionIntent, error) {
	return scanSubmissionIntent(q.db.QueryRow(ctx, claimNextSubmissionIntent, workerID))
}

type BeginSubmissionParams struct {
	ID              pgtype.UUID
	WorkerID        string
	LeaseGeneration int64
}

const beginSubmission = `
WITH updated AS (
    UPDATE submission_intents
    SET status = 'SUBMITTING', updated_at = now()
    WHERE id = $1
      AND status = 'CLAIMED'
      AND lease_owner = $2
      AND lease_generation = $3
      AND lease_expires_at > now()
    RETURNING id, user_id, application_id, package_id, approval_id, package_hash,
              idempotency_key, status, lease_generation, lease_owner, lease_expires_at,
              attempt_count, last_error, created_at, updated_at, completed_at
), attempt_update AS (
    UPDATE submission_attempts a
    SET state = 'SUBMITTING', submitting_at = now()
    FROM updated u
    WHERE a.intent_id = u.id
      AND a.lease_generation = u.lease_generation
    RETURNING a.intent_id
)
SELECT u.* FROM updated u JOIN attempt_update a ON a.intent_id = u.id
`

func (q *Queries) BeginSubmission(ctx context.Context, arg BeginSubmissionParams) (SubmissionIntent, error) {
	return scanSubmissionIntent(q.db.QueryRow(ctx, beginSubmission, arg.ID, arg.WorkerID, arg.LeaseGeneration))
}

type ConfirmSubmissionParams struct {
	ID              pgtype.UUID
	WorkerID        string
	LeaseGeneration int64
	ReceiptJson     []byte
}

const confirmSubmission = `
WITH updated AS (
    UPDATE submission_intents
    SET status = 'CONFIRMED',
        completed_at = now(),
        lease_owner = NULL,
        lease_expires_at = NULL,
        updated_at = now()
    WHERE id = $1
      AND status = 'SUBMITTING'
      AND lease_owner = $2
      AND lease_generation = $3
      AND lease_expires_at > now()
    RETURNING id, user_id, application_id, package_id, approval_id, package_hash,
              idempotency_key, status, lease_generation, lease_owner, lease_expires_at,
              attempt_count, last_error, created_at, updated_at, completed_at
), attempt_update AS (
    UPDATE submission_attempts a
    SET state = 'CONFIRMED', receipt_json = $4, finished_at = now()
    FROM updated u
    WHERE a.intent_id = u.id
      AND a.lease_generation = u.lease_generation
    RETURNING a.intent_id
)
SELECT u.* FROM updated u JOIN attempt_update a ON a.intent_id = u.id
`

func (q *Queries) ConfirmSubmission(ctx context.Context, arg ConfirmSubmissionParams) (SubmissionIntent, error) {
	return scanSubmissionIntent(q.db.QueryRow(ctx, confirmSubmission, arg.ID, arg.WorkerID, arg.LeaseGeneration, arg.ReceiptJson))
}

type MarkSubmissionUncertainParams struct {
	ID              pgtype.UUID
	WorkerID        string
	LeaseGeneration int64
	ErrorMessage    string
}

const markSubmissionUncertain = `
WITH updated AS (
    UPDATE submission_intents
    SET status = 'UNCERTAIN',
        last_error = $4,
        lease_owner = NULL,
        lease_expires_at = NULL,
        updated_at = now()
    WHERE id = $1
      AND status = 'SUBMITTING'
      AND lease_owner = $2
      AND lease_generation = $3
    RETURNING id, user_id, application_id, package_id, approval_id, package_hash,
              idempotency_key, status, lease_generation, lease_owner, lease_expires_at,
              attempt_count, last_error, created_at, updated_at, completed_at
), attempt_update AS (
    UPDATE submission_attempts a
    SET state = 'UNCERTAIN', error_message = $4, finished_at = now()
    FROM updated u
    WHERE a.intent_id = u.id
      AND a.lease_generation = u.lease_generation
    RETURNING a.intent_id
)
SELECT u.* FROM updated u JOIN attempt_update a ON a.intent_id = u.id
`

func (q *Queries) MarkSubmissionUncertain(ctx context.Context, arg MarkSubmissionUncertainParams) (SubmissionIntent, error) {
	return scanSubmissionIntent(q.db.QueryRow(ctx, markSubmissionUncertain, arg.ID, arg.WorkerID, arg.LeaseGeneration, arg.ErrorMessage))
}
