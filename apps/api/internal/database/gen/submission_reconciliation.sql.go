package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type ReconcileSubmissionAsAppliedParams struct {
	ID     pgtype.UUID
	UserID pgtype.UUID
	Note   string
}

const reconcileSubmissionAsApplied = `
WITH updated AS (
    UPDATE submission_intents
    SET status = 'CONFIRMED', last_error = NULL, completed_at = now(),
        lease_owner = NULL, lease_expires_at = NULL, updated_at = now()
    WHERE id = $1 AND user_id = $2 AND status = 'UNCERTAIN'
    RETURNING id, user_id, application_id, package_id, approval_id, package_hash,
              idempotency_key, status, lease_generation, lease_owner, lease_expires_at,
              attempt_count, last_error, created_at, updated_at, completed_at
), application_current AS (
    SELECT a.id, a.status FROM applications a JOIN updated u ON u.application_id = a.id FOR UPDATE OF a
), application_updated AS (
    UPDATE applications a
    SET status = 'APPLIED', applied_at = COALESCE(a.applied_at, now()), updated_at = now()
    FROM application_current c WHERE a.id = c.id
    RETURNING a.id, c.status AS from_status
), event_write AS (
    INSERT INTO application_events (application_id, event_type, from_status, to_status, notes)
    SELECT id, 'SUBMISSION_RECONCILED', from_status, 'APPLIED', $3 FROM application_updated
    RETURNING application_id
), attempt_update AS (
    UPDATE submission_attempts a
    SET state = 'CONFIRMED', receipt_json = jsonb_build_object('source', 'manual_reconciliation', 'note', $3),
        error_message = NULL, finished_at = now()
    FROM updated u
    WHERE a.intent_id = u.id AND a.lease_generation = u.lease_generation
    RETURNING a.intent_id
)
SELECT u.* FROM updated u
JOIN application_updated au ON au.id = u.application_id
JOIN attempt_update att ON att.intent_id = u.id
LEFT JOIN event_write e ON e.application_id = u.application_id
`

func (q *Queries) ReconcileSubmissionAsApplied(ctx context.Context, arg ReconcileSubmissionAsAppliedParams) (SubmissionIntent, error) {
	return scanSubmissionIntent(q.db.QueryRow(ctx, reconcileSubmissionAsApplied, arg.ID, arg.UserID, arg.Note))
}

type ReconcileSubmissionAsNotSubmittedParams struct {
	ID     pgtype.UUID
	UserID pgtype.UUID
	Note   string
}

const reconcileSubmissionAsNotSubmitted = `
WITH updated AS (
    UPDATE submission_intents
    SET status = 'CANCELLED', last_error = $3, completed_at = now(),
        lease_owner = NULL, lease_expires_at = NULL, updated_at = now()
    WHERE id = $1 AND user_id = $2 AND status = 'UNCERTAIN'
    RETURNING id, user_id, application_id, package_id, approval_id, package_hash,
              idempotency_key, status, lease_generation, lease_owner, lease_expires_at,
              attempt_count, last_error, created_at, updated_at, completed_at
), attempt_update AS (
    UPDATE submission_attempts a
    SET state = 'FAILED', error_message = $3, finished_at = now()
    FROM updated u
    WHERE a.intent_id = u.id AND a.lease_generation = u.lease_generation
    RETURNING a.intent_id
)
SELECT u.* FROM updated u JOIN attempt_update a ON a.intent_id = u.id
`

func (q *Queries) ReconcileSubmissionAsNotSubmitted(ctx context.Context, arg ReconcileSubmissionAsNotSubmittedParams) (SubmissionIntent, error) {
	return scanSubmissionIntent(q.db.QueryRow(ctx, reconcileSubmissionAsNotSubmitted, arg.ID, arg.UserID, arg.Note))
}

type ReactivateCancelledSubmissionIntentParams struct {
	ID     pgtype.UUID
	UserID pgtype.UUID
}

const reactivateCancelledSubmissionIntent = `
WITH active_approval AS (
    SELECT a.id
    FROM application_approvals a
    JOIN submission_intents i ON i.package_id = a.package_id
    WHERE i.id = $1 AND i.user_id = $2
      AND a.user_id = i.user_id AND a.package_hash = i.package_hash
      AND a.action_scope = 'SUBMIT_ONCE' AND a.revoked_at IS NULL AND a.expires_at > now()
    ORDER BY a.approved_at DESC LIMIT 1
)
UPDATE submission_intents i
SET status = 'PENDING', approval_id = a.id, lease_owner = NULL, lease_expires_at = NULL,
    last_error = NULL, completed_at = NULL, updated_at = now()
FROM active_approval a
WHERE i.id = $1 AND i.user_id = $2 AND i.status IN ('CANCELLED', 'FAILED')
RETURNING i.id, i.user_id, i.application_id, i.package_id, i.approval_id, i.package_hash,
          i.idempotency_key, i.status, i.lease_generation, i.lease_owner, i.lease_expires_at,
          i.attempt_count, i.last_error, i.created_at, i.updated_at, i.completed_at
`

func (q *Queries) ReactivateCancelledSubmissionIntent(ctx context.Context, arg ReactivateCancelledSubmissionIntentParams) (SubmissionIntent, error) {
	return scanSubmissionIntent(q.db.QueryRow(ctx, reactivateCancelledSubmissionIntent, arg.ID, arg.UserID))
}
