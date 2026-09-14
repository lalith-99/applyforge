package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type ApplicationPackage struct {
	ID                pgtype.UUID        `json:"id"`
	UserID            pgtype.UUID        `json:"user_id"`
	ApplicationID     pgtype.UUID        `json:"application_id"`
	JobID             pgtype.UUID        `json:"job_id"`
	ResumeVersionID   pgtype.UUID        `json:"resume_version_id"`
	DestinationUrl    string             `json:"destination_url"`
	DestinationOrigin string             `json:"destination_origin"`
	Adapter           string             `json:"adapter"`
	FormVersion       string             `json:"form_version"`
	AnswersJson       []byte             `json:"answers_json"`
	RequiredFields    []byte             `json:"required_fields"`
	EvidenceRefs      []byte             `json:"evidence_refs"`
	ResumeContentHash string             `json:"resume_content_hash"`
	AnswersHash       string             `json:"answers_hash"`
	PackageHash       string             `json:"package_hash"`
	CreatedAt         pgtype.Timestamptz `json:"created_at"`
}

type CreateApplicationPackageParams struct {
	UserID            pgtype.UUID
	ApplicationID     pgtype.UUID
	JobID             pgtype.UUID
	ResumeVersionID   pgtype.UUID
	DestinationUrl    string
	DestinationOrigin string
	Adapter           string
	FormVersion       string
	AnswersJson       []byte
	RequiredFields    []byte
	EvidenceRefs      []byte
	ResumeContentHash string
	AnswersHash       string
	PackageHash       string
}

const createApplicationPackage = `
INSERT INTO application_packages (
    user_id, application_id, job_id, resume_version_id, destination_url, destination_origin,
    adapter, form_version, answers_json, required_fields, evidence_refs,
    resume_content_hash, answers_hash, package_hash
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
ON CONFLICT (user_id, application_id, package_hash) DO UPDATE
SET package_hash = application_packages.package_hash
RETURNING id, user_id, application_id, job_id, resume_version_id, destination_url,
          destination_origin, adapter, form_version, answers_json, required_fields,
          evidence_refs, resume_content_hash, answers_hash, package_hash, created_at
`

func (q *Queries) CreateApplicationPackage(ctx context.Context, arg CreateApplicationPackageParams) (ApplicationPackage, error) {
	row := q.db.QueryRow(ctx, createApplicationPackage,
		arg.UserID, arg.ApplicationID, arg.JobID, arg.ResumeVersionID,
		arg.DestinationUrl, arg.DestinationOrigin, arg.Adapter, arg.FormVersion,
		arg.AnswersJson, arg.RequiredFields, arg.EvidenceRefs,
		arg.ResumeContentHash, arg.AnswersHash, arg.PackageHash,
	)
	var i ApplicationPackage
	err := row.Scan(
		&i.ID, &i.UserID, &i.ApplicationID, &i.JobID, &i.ResumeVersionID,
		&i.DestinationUrl, &i.DestinationOrigin, &i.Adapter, &i.FormVersion,
		&i.AnswersJson, &i.RequiredFields, &i.EvidenceRefs,
		&i.ResumeContentHash, &i.AnswersHash, &i.PackageHash, &i.CreatedAt,
	)
	return i, err
}

type GetApplicationPackageForUserParams struct {
	ID     pgtype.UUID
	UserID pgtype.UUID
}

const getApplicationPackageForUser = `
SELECT id, user_id, application_id, job_id, resume_version_id, destination_url,
       destination_origin, adapter, form_version, answers_json, required_fields,
       evidence_refs, resume_content_hash, answers_hash, package_hash, created_at
FROM application_packages
WHERE id = $1 AND user_id = $2
`

func (q *Queries) GetApplicationPackageForUser(ctx context.Context, arg GetApplicationPackageForUserParams) (ApplicationPackage, error) {
	row := q.db.QueryRow(ctx, getApplicationPackageForUser, arg.ID, arg.UserID)
	var i ApplicationPackage
	err := row.Scan(
		&i.ID, &i.UserID, &i.ApplicationID, &i.JobID, &i.ResumeVersionID,
		&i.DestinationUrl, &i.DestinationOrigin, &i.Adapter, &i.FormVersion,
		&i.AnswersJson, &i.RequiredFields, &i.EvidenceRefs,
		&i.ResumeContentHash, &i.AnswersHash, &i.PackageHash, &i.CreatedAt,
	)
	return i, err
}

type ApplicationApproval struct {
	ID                  pgtype.UUID        `json:"id"`
	PackageID           pgtype.UUID        `json:"package_id"`
	UserID              pgtype.UUID        `json:"user_id"`
	PackageHash         string             `json:"package_hash"`
	ActionScope         string             `json:"action_scope"`
	ConfirmationVersion string             `json:"confirmation_version"`
	ConfirmationText    string             `json:"confirmation_text"`
	ApprovedAt          pgtype.Timestamptz `json:"approved_at"`
	ExpiresAt           pgtype.Timestamptz `json:"expires_at"`
	RevokedAt           pgtype.Timestamptz `json:"revoked_at"`
}

type UpsertApplicationApprovalParams struct {
	PackageID           pgtype.UUID
	UserID              pgtype.UUID
	PackageHash         string
	ActionScope         string
	ConfirmationVersion string
	ConfirmationText    string
	ExpiresAt           pgtype.Timestamptz
}

const upsertApplicationApproval = `
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
RETURNING id, package_id, user_id, package_hash, action_scope, confirmation_version,
          confirmation_text, approved_at, expires_at, revoked_at
`

func (q *Queries) UpsertApplicationApproval(ctx context.Context, arg UpsertApplicationApprovalParams) (ApplicationApproval, error) {
	row := q.db.QueryRow(ctx, upsertApplicationApproval,
		arg.PackageID, arg.UserID, arg.PackageHash, arg.ActionScope,
		arg.ConfirmationVersion, arg.ConfirmationText, arg.ExpiresAt,
	)
	var i ApplicationApproval
	err := row.Scan(
		&i.ID, &i.PackageID, &i.UserID, &i.PackageHash, &i.ActionScope,
		&i.ConfirmationVersion, &i.ConfirmationText, &i.ApprovedAt, &i.ExpiresAt, &i.RevokedAt,
	)
	return i, err
}

type RevokeApplicationApprovalParams struct {
	PackageID pgtype.UUID
	UserID    pgtype.UUID
}

const revokeApplicationApproval = `
UPDATE application_approvals
SET revoked_at = now()
WHERE package_id = $1 AND user_id = $2 AND revoked_at IS NULL
RETURNING id, package_id, user_id, package_hash, action_scope, confirmation_version,
          confirmation_text, approved_at, expires_at, revoked_at
`

func (q *Queries) RevokeApplicationApproval(ctx context.Context, arg RevokeApplicationApprovalParams) (ApplicationApproval, error) {
	row := q.db.QueryRow(ctx, revokeApplicationApproval, arg.PackageID, arg.UserID)
	var i ApplicationApproval
	err := row.Scan(
		&i.ID, &i.PackageID, &i.UserID, &i.PackageHash, &i.ActionScope,
		&i.ConfirmationVersion, &i.ConfirmationText, &i.ApprovedAt, &i.ExpiresAt, &i.RevokedAt,
	)
	return i, err
}
