package applications

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

const (
	ApprovalActionScope         = "SUBMIT_ONCE"
	ApprovalConfirmationVersion = "submit-once-v1"
	ApprovalConfirmationText    = "I approve submitting this exact application package once."
	approvalLifetime            = 24 * time.Hour
	packageAdapter              = "BROWSER_COMPANION"
	packageFormVersion          = "v1"
)

var (
	ErrApplicationNotReady       = errors.New("application is not ready to apply")
	ErrMissingResumeVersion      = errors.New("application has no resume version")
	ErrMissingDestination        = errors.New("job has no safe application destination")
	ErrApplicationPackageMissing = errors.New("application package not found")
	ErrInvalidApproval           = errors.New("invalid application approval confirmation")
)

// ApplicationPackage is the immutable snapshot that an executor is allowed to
// submit. Changing the resume, answers, destination, or adapter creates a new
// package hash and therefore requires a new approval.
type ApplicationPackage struct {
	ID                uuid.UUID       `json:"id"`
	ApplicationID     uuid.UUID       `json:"application_id"`
	JobID             uuid.UUID       `json:"job_id"`
	ResumeVersionID   uuid.UUID       `json:"resume_version_id"`
	DestinationURL    string          `json:"destination_url"`
	DestinationOrigin string          `json:"destination_origin"`
	Adapter           string          `json:"adapter"`
	FormVersion       string          `json:"form_version"`
	AnswersJSON       json.RawMessage `json:"answers_json"`
	RequiredFields    json.RawMessage `json:"required_fields"`
	EvidenceRefs      json.RawMessage `json:"evidence_refs"`
	ResumeContentHash string          `json:"resume_content_hash"`
	AnswersHash       string          `json:"answers_hash"`
	PackageHash       string          `json:"package_hash"`
	CreatedAt         time.Time       `json:"created_at"`
}

// ApplicationApproval authorizes exactly one immutable package for one submit
// action. It is intentionally short-lived and can be revoked before execution.
type ApplicationApproval struct {
	ID                  uuid.UUID  `json:"id"`
	PackageID           uuid.UUID  `json:"package_id"`
	PackageHash         string     `json:"package_hash"`
	ActionScope         string     `json:"action_scope"`
	ConfirmationVersion string     `json:"confirmation_version"`
	ConfirmationText    string     `json:"confirmation_text"`
	ApprovedAt          time.Time  `json:"approved_at"`
	ExpiresAt           time.Time  `json:"expires_at"`
	RevokedAt           *time.Time `json:"revoked_at,omitempty"`
}

func packageFromRow(row db.ApplicationPackage) ApplicationPackage {
	return ApplicationPackage{
		ID:                database.PGToUUID(row.ID),
		ApplicationID:     database.PGToUUID(row.ApplicationID),
		JobID:             database.PGToUUID(row.JobID),
		ResumeVersionID:   database.PGToUUID(row.ResumeVersionID),
		DestinationURL:    row.DestinationUrl,
		DestinationOrigin: row.DestinationOrigin,
		Adapter:           row.Adapter,
		FormVersion:       row.FormVersion,
		AnswersJSON:       append(json.RawMessage(nil), row.AnswersJson...),
		RequiredFields:    append(json.RawMessage(nil), row.RequiredFields...),
		EvidenceRefs:      append(json.RawMessage(nil), row.EvidenceRefs...),
		ResumeContentHash: row.ResumeContentHash,
		AnswersHash:       row.AnswersHash,
		PackageHash:       row.PackageHash,
		CreatedAt:         row.CreatedAt.Time,
	}
}

func approvalFromRow(row db.ApplicationApproval) ApplicationApproval {
	var revokedAt *time.Time
	if row.RevokedAt.Valid {
		t := row.RevokedAt.Time
		revokedAt = &t
	}
	return ApplicationApproval{
		ID:                  database.PGToUUID(row.ID),
		PackageID:           database.PGToUUID(row.PackageID),
		PackageHash:         row.PackageHash,
		ActionScope:         row.ActionScope,
		ConfirmationVersion: row.ConfirmationVersion,
		ConfirmationText:    row.ConfirmationText,
		ApprovedAt:          row.ApprovedAt.Time,
		ExpiresAt:           row.ExpiresAt.Time,
		RevokedAt:           revokedAt,
	}
}

type answerSnapshot struct {
	FullName          *string         `json:"full_name,omitempty"`
	Phone             *string         `json:"phone,omitempty"`
	Email             *string         `json:"email,omitempty"`
	Location          *string         `json:"location,omitempty"`
	DesiredLocation   *string         `json:"desired_location,omitempty"`
	WorkAuthorization *string         `json:"work_authorization,omitempty"`
	Sponsorship       *string         `json:"sponsorship,omitempty"`
	SalaryExpectation *string         `json:"salary_expectation,omitempty"`
	NoticePeriod      *string         `json:"notice_period,omitempty"`
	LinkedinURL       *string         `json:"linkedin_url,omitempty"`
	GithubURL         *string         `json:"github_url,omitempty"`
	PortfolioURL      *string         `json:"portfolio_url,omitempty"`
	CommonAnswers     json.RawMessage `json:"common_answers"`
}

type packageHashInput struct {
	ApplicationID     string          `json:"application_id"`
	JobID             string          `json:"job_id"`
	ResumeVersionID   string          `json:"resume_version_id"`
	DestinationURL    string          `json:"destination_url"`
	DestinationOrigin string          `json:"destination_origin"`
	Adapter           string          `json:"adapter"`
	FormVersion       string          `json:"form_version"`
	AnswersJSON       json.RawMessage `json:"answers_json"`
	RequiredFields    json.RawMessage `json:"required_fields"`
	EvidenceRefs      json.RawMessage `json:"evidence_refs"`
	ResumeContentHash string          `json:"resume_content_hash"`
	AnswersHash       string          `json:"answers_hash"`
}

// BuildApplicationPackage snapshots only server-owned records. Client-provided
// resume contents, answers, and destinations are deliberately not accepted.
func (s *Service) BuildApplicationPackage(ctx context.Context, userID, applicationID uuid.UUID) (ApplicationPackage, error) {
	app, err := s.repo.GetForUser(ctx, applicationID, userID)
	if err != nil {
		return ApplicationPackage{}, err
	}
	if app.Status != StatusReadyToApply {
		return ApplicationPackage{}, ErrApplicationNotReady
	}
	if app.ResumeVersionID == nil {
		return ApplicationPackage{}, ErrMissingResumeVersion
	}
	if err := s.repo.ValidateResumeVersionForJob(ctx, userID, app.JobID, app.ResumeVersionID); err != nil {
		return ApplicationPackage{}, err
	}

	version, err := s.repo.q.GetResumeVersionForUser(ctx, db.GetResumeVersionForUserParams{
		ID: database.UUIDToPG(*app.ResumeVersionID), UserID: database.UUIDToPG(userID),
	})
	if err != nil {
		return ApplicationPackage{}, err
	}
	job, err := s.repo.q.GetJobByID(ctx, database.UUIDToPG(app.JobID))
	if err != nil {
		return ApplicationPackage{}, err
	}

	destination, origin, err := safeDestination(job.ApplyUrl, job.SourceUrl)
	if err != nil {
		return ApplicationPackage{}, err
	}
	answersJSON, err := s.snapshotAnswers(ctx, userID)
	if err != nil {
		return ApplicationPackage{}, err
	}

	resumeHash := sha256Hex(version.ContentJson)
	answersHash := sha256Hex(answersJSON)
	requiredFields := json.RawMessage(`[]`)
	evidenceRefs := json.RawMessage(`[]`)

	hashPayload, err := json.Marshal(packageHashInput{
		ApplicationID: applicationID.String(), JobID: app.JobID.String(), ResumeVersionID: app.ResumeVersionID.String(),
		DestinationURL: destination, DestinationOrigin: origin, Adapter: packageAdapter, FormVersion: packageFormVersion,
		AnswersJSON: answersJSON, RequiredFields: requiredFields, EvidenceRefs: evidenceRefs,
		ResumeContentHash: resumeHash, AnswersHash: answersHash,
	})
	if err != nil {
		return ApplicationPackage{}, err
	}
	packageHash := sha256Hex(hashPayload)

	row, err := s.repo.q.CreateApplicationPackage(ctx, db.CreateApplicationPackageParams{
		UserID: database.UUIDToPG(userID), ApplicationID: database.UUIDToPG(applicationID), JobID: database.UUIDToPG(app.JobID),
		ResumeVersionID: database.UUIDToPG(*app.ResumeVersionID), DestinationUrl: destination, DestinationOrigin: origin,
		Adapter: packageAdapter, FormVersion: packageFormVersion, AnswersJson: answersJSON, RequiredFields: requiredFields,
		EvidenceRefs: evidenceRefs, ResumeContentHash: resumeHash, AnswersHash: answersHash, PackageHash: packageHash,
	})
	if err != nil {
		return ApplicationPackage{}, err
	}
	return packageFromRow(row), nil
}

func (s *Service) GetApplicationPackage(ctx context.Context, userID, packageID uuid.UUID) (ApplicationPackage, error) {
	row, err := s.repo.q.GetApplicationPackageForUser(ctx, db.GetApplicationPackageForUserParams{
		ID: database.UUIDToPG(packageID), UserID: database.UUIDToPG(userID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ApplicationPackage{}, ErrApplicationPackageMissing
		}
		return ApplicationPackage{}, err
	}
	return packageFromRow(row), nil
}

func (s *Service) ApproveApplicationPackage(ctx context.Context, userID, packageID uuid.UUID, confirmationVersion, confirmationText string) (ApplicationApproval, error) {
	if confirmationVersion != ApprovalConfirmationVersion || confirmationText != ApprovalConfirmationText {
		return ApplicationApproval{}, ErrInvalidApproval
	}
	pkg, err := s.GetApplicationPackage(ctx, userID, packageID)
	if err != nil {
		return ApplicationApproval{}, err
	}
	app, err := s.repo.GetForUser(ctx, pkg.ApplicationID, userID)
	if err != nil {
		return ApplicationApproval{}, err
	}
	if app.Status != StatusReadyToApply {
		return ApplicationApproval{}, ErrApplicationNotReady
	}

	expiresAt := time.Now().UTC().Add(approvalLifetime)
	row, err := s.repo.q.UpsertApplicationApproval(ctx, db.UpsertApplicationApprovalParams{
		PackageID: database.UUIDToPG(packageID), UserID: database.UUIDToPG(userID), PackageHash: pkg.PackageHash,
		ActionScope: ApprovalActionScope, ConfirmationVersion: confirmationVersion, ConfirmationText: confirmationText,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return ApplicationApproval{}, err
	}
	note := "package_id=" + packageID.String() + " package_hash=" + pkg.PackageHash
	_, _ = s.repo.CreateEvent(ctx, pkg.ApplicationID, "APPLICATION_PACKAGE_APPROVED", nil, nil, &note)
	return approvalFromRow(row), nil
}

func (s *Service) RevokeApplicationPackageApproval(ctx context.Context, userID, packageID uuid.UUID) (ApplicationApproval, error) {
	pkg, err := s.GetApplicationPackage(ctx, userID, packageID)
	if err != nil {
		return ApplicationApproval{}, err
	}
	row, err := s.repo.q.RevokeApplicationApproval(ctx, db.RevokeApplicationApprovalParams{
		PackageID: database.UUIDToPG(packageID), UserID: database.UUIDToPG(userID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ApplicationApproval{}, ErrApplicationPackageMissing
		}
		return ApplicationApproval{}, err
	}
	note := "package_id=" + packageID.String() + " package_hash=" + pkg.PackageHash
	_, _ = s.repo.CreateEvent(ctx, pkg.ApplicationID, "APPLICATION_PACKAGE_APPROVAL_REVOKED", nil, nil, &note)
	return approvalFromRow(row), nil
}

func (s *Service) snapshotAnswers(ctx context.Context, userID uuid.UUID) (json.RawMessage, error) {
	answers, err := s.repo.GetAnswers(ctx, userID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if errors.Is(err, ErrNotFound) {
		return json.RawMessage(`{"common_answers":{}}`), nil
	}

	common := canonicalJSON(answers.CommonAnswers)
	snapshot := answerSnapshot{
		FullName: answers.FullName, Phone: answers.Phone, Email: answers.Email, Location: answers.Location,
		DesiredLocation: answers.DesiredLocation, WorkAuthorization: answers.WorkAuthorization, Sponsorship: answers.Sponsorship,
		SalaryExpectation: answers.SalaryExpectation, NoticePeriod: answers.NoticePeriod, LinkedinURL: answers.LinkedinURL,
		GithubURL: answers.GithubURL, PortfolioURL: answers.PortfolioURL, CommonAnswers: common,
	}
	payload, err := json.Marshal(snapshot)
	return json.RawMessage(payload), err
}

func safeDestination(primary, fallback pgtype.Text) (string, string, error) {
	for _, candidate := range []pgtype.Text{primary, fallback} {
		if !candidate.Valid || strings.TrimSpace(candidate.String) == "" {
			continue
		}
		parsed, err := url.Parse(strings.TrimSpace(candidate.String))
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
			continue
		}
		return parsed.String(), parsed.Scheme + "://" + parsed.Host, nil
	}
	return "", "", ErrMissingDestination
}

func canonicalJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return json.RawMessage(`{}`)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return canonical
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
