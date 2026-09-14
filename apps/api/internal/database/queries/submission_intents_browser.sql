-- name: ClaimSubmissionIntentForUser :one
WITH expired AS (
    UPDATE submission_intents i
    SET status = CASE
            WHEN i.status = 'SUBMITTING' THEN 'UNCERTAIN'
            WHEN EXISTS (
                SELECT 1 FROM application_approvals a
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
        last_error = CASE
            WHEN i.status = 'SUBMITTING' THEN 'submission lease expired after irreversible phase began'
            WHEN NOT EXISTS (
                SELECT 1 FROM application_approvals a
                WHERE a.id = i.approval_id
                  AND a.package_id = i.package_id
                  AND a.user_id = i.user_id
                  AND a.package_hash = i.package_hash
                  AND a.action_scope = 'SUBMIT_ONCE'
                  AND a.revoked_at IS NULL
                  AND a.expires_at > now()
            ) THEN 'application approval expired or was revoked before submission began'
            ELSE i.last_error
        END,
        lease_owner = NULL,
        lease_expires_at = NULL,
        updated_at = now()
    WHERE i.id = $1 AND i.user_id = $2
      AND i.status IN ('CLAIMED', 'SUBMITTING')
      AND i.lease_expires_at < now()
), claimed AS (
    UPDATE submission_intents i
    SET status = 'CLAIMED',
        lease_generation = i.lease_generation + 1,
        lease_owner = $3,
        lease_expires_at = now() + interval '5 minutes',
        attempt_count = i.attempt_count + 1,
        last_error = NULL,
        updated_at = now()
    WHERE i.id = $1
      AND i.user_id = $2
      AND i.status = 'PENDING'
      AND EXISTS (
          SELECT 1 FROM application_approvals a
          WHERE a.id = i.approval_id
            AND a.package_id = i.package_id
            AND a.user_id = i.user_id
            AND a.package_hash = i.package_hash
            AND a.action_scope = 'SUBMIT_ONCE'
            AND a.revoked_at IS NULL
            AND a.expires_at > now()
      )
    RETURNING i.id, i.user_id, i.application_id, i.package_id, i.approval_id, i.package_hash,
              i.idempotency_key, i.status, i.lease_generation, i.lease_owner, i.lease_expires_at,
              i.attempt_count, i.last_error, i.created_at, i.updated_at, i.completed_at
), attempt_write AS (
    INSERT INTO submission_attempts (intent_id, lease_generation, worker_id, state)
    SELECT id, lease_generation, lease_owner, 'CLAIMED'
    FROM claimed
    RETURNING intent_id
)
SELECT c.* FROM claimed c JOIN attempt_write a ON a.intent_id = c.id;
