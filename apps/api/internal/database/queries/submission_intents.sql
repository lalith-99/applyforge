-- name: CreateSubmissionIntent :one
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
RETURNING *;

-- name: GetSubmissionIntentForUser :one
SELECT * FROM submission_intents WHERE id = $1 AND user_id = $2;

-- name: ClaimNextSubmissionIntent :one
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
    UPDATE submission_intents i
    SET status = CASE
            WHEN EXISTS (
                SELECT 1
                FROM application_approvals a
                WHERE a.id = i.approval_id
                  AND a.package_id = i.package_id
                  AND a.user_id = i.user_id
                  AND a.package_hash = i.package_hash
                  AND a.action_scope = 'SUBMIT_ONCE'
                  AND a.revoked_at IS NULL
                  AND a.expires_at > now()
            ) THEN 'PENDING'
            ELSE 'CANCELLED'
        END,
        lease_owner = NULL,
        lease_expires_at = NULL,
        updated_at = now()
    WHERE i.status = 'CLAIMED'
      AND i.lease_expires_at < now()
), invalid_authorizations AS (
    UPDATE submission_intents i
    SET status = 'CANCELLED',
        last_error = 'application approval expired or was revoked before submission began',
        updated_at = now()
    WHERE i.status = 'PENDING'
      AND NOT EXISTS (
          SELECT 1
          FROM application_approvals a
          WHERE a.id = i.approval_id
            AND a.package_id = i.package_id
            AND a.user_id = i.user_id
            AND a.package_hash = i.package_hash
            AND a.action_scope = 'SUBMIT_ONCE'
            AND a.revoked_at IS NULL
            AND a.expires_at > now()
      )
), candidate AS (
    SELECT i.id
    FROM submission_intents i
    JOIN application_approvals a ON a.id = i.approval_id
    WHERE i.status = 'PENDING'
      AND a.package_id = i.package_id
      AND a.user_id = i.user_id
      AND a.package_hash = i.package_hash
      AND a.action_scope = 'SUBMIT_ONCE'
      AND a.revoked_at IS NULL
      AND a.expires_at > now()
    ORDER BY i.created_at ASC
    LIMIT 1
    FOR UPDATE OF i SKIP LOCKED
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
    RETURNING i.*
), attempt_write AS (
    INSERT INTO submission_attempts (intent_id, lease_generation, worker_id, state)
    SELECT id, lease_generation, lease_owner, 'CLAIMED'
    FROM claimed
    RETURNING intent_id
)
SELECT c.*
FROM claimed c
JOIN attempt_write a ON a.intent_id = c.id;

-- name: BeginSubmission :one
WITH updated AS (
    UPDATE submission_intents i
    SET status = 'SUBMITTING', updated_at = now()
    WHERE i.id = $1
      AND i.status = 'CLAIMED'
      AND i.lease_owner = $2
      AND i.lease_generation = $3
      AND i.lease_expires_at > now()
      AND EXISTS (
          SELECT 1
          FROM application_approvals a
          WHERE a.id = i.approval_id
            AND a.package_id = i.package_id
            AND a.user_id = i.user_id
            AND a.package_hash = i.package_hash
            AND a.action_scope = 'SUBMIT_ONCE'
            AND a.revoked_at IS NULL
            AND a.expires_at > now()
      )
    RETURNING i.*
), attempt_update AS (
    UPDATE submission_attempts a
    SET state = 'SUBMITTING', submitting_at = now()
    FROM updated u
    WHERE a.intent_id = u.id
      AND a.lease_generation = u.lease_generation
    RETURNING a.intent_id
)
SELECT u.* FROM updated u JOIN attempt_update a ON a.intent_id = u.id;

-- name: ConfirmSubmission :one
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
    RETURNING *
), application_current AS (
    SELECT a.id, a.status
    FROM applications a
    JOIN updated u ON u.application_id = a.id
    FOR UPDATE OF a
), application_updated AS (
    UPDATE applications a
    SET status = 'APPLIED',
        applied_at = COALESCE(a.applied_at, now()),
        updated_at = now()
    FROM application_current c
    WHERE a.id = c.id
    RETURNING a.id, c.status AS from_status
), event_write AS (
    INSERT INTO application_events (application_id, event_type, from_status, to_status, notes)
    SELECT id, 'SUBMISSION_CONFIRMED', from_status, 'APPLIED', 'submission receipt confirmed'
    FROM application_updated
    WHERE from_status <> 'APPLIED'
    RETURNING application_id
), attempt_update AS (
    UPDATE submission_attempts a
    SET state = 'CONFIRMED', receipt_json = $4, finished_at = now()
    FROM updated u
    WHERE a.intent_id = u.id
      AND a.lease_generation = u.lease_generation
    RETURNING a.intent_id
)
SELECT u.*
FROM updated u
JOIN attempt_update a ON a.intent_id = u.id
JOIN application_updated au ON au.id = u.application_id
LEFT JOIN event_write e ON e.application_id = u.application_id;

-- name: MarkSubmissionUncertain :one
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
    RETURNING *
), attempt_update AS (
    UPDATE submission_attempts a
    SET state = 'UNCERTAIN', error_message = $4, finished_at = now()
    FROM updated u
    WHERE a.intent_id = u.id
      AND a.lease_generation = u.lease_generation
    RETURNING a.intent_id
)
SELECT u.* FROM updated u JOIN attempt_update a ON a.intent_id = u.id;
