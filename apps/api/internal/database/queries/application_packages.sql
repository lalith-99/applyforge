-- name: CreateApplicationPackage :one
INSERT INTO application_packages (
    user_id, application_id, job_id, resume_version_id, destination_url, destination_origin,
    adapter, form_version, answers_json, required_fields, evidence_refs,
    resume_content_hash, answers_hash, package_hash
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
ON CONFLICT (user_id, application_id, package_hash) DO UPDATE
SET package_hash = application_packages.package_hash
RETURNING *;

-- name: GetApplicationPackageForUser :one
SELECT *
FROM application_packages
WHERE id = $1 AND user_id = $2;

-- name: UpsertApplicationApproval :one
INSERT INTO application_approvals (
    package_id, user_id, package_hash, action_scope, confirmation_version,
    confirmation_text, expires_at
) VALUES ($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT (package_id, user_id, action_scope) DO UPDATE SET
    package_hash = EXCLUDED.package_hash,
    confirmation_version = EXCLUDED.confirmation_version,
    confirmation_text = EXCLUDED.confirmation_text,
    approved_at = now(),
    expires_at = EXCLUDED.expires_at,
    revoked_at = NULL
RETURNING *;

-- name: RevokeApplicationApproval :one
UPDATE application_approvals
SET revoked_at = now()
WHERE package_id = $1 AND user_id = $2 AND revoked_at IS NULL
RETURNING *;
