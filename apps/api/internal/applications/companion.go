package applications

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

const companionTokenLifetime = 20 * time.Minute

var (
	ErrCompanionUnauthorized     = errors.New("invalid or expired companion token")
	ErrCompanionResumeUnavailable = errors.New("approved resume PDF is unavailable")
)

// StorageGetter is the only object-storage capability the browser companion
// requires. The raw token never grants arbitrary storage access; only the PDF
// referenced by the already-approved package can be read.
type StorageGetter interface {
	Get(ctx context.Context, key string) ([]byte, error)
}

// CompanionService coordinates the short-lived capability handed from the web
// UI to the browser extension. Normal user sessions mint the capability; ATS
// pages use only the capability and never receive the user's session cookie.
type CompanionService struct {
	applications *Service
	repo         *Repository
	storage      StorageGetter
}

func NewCompanionService(applications *Service, repo *Repository, storage StorageGetter) *CompanionService {
	return &CompanionService{applications: applications, repo: repo, storage: storage}
}

type CompanionHandoff struct {
	IntentID  uuid.UUID `json:"intent_id"`
	PackageID uuid.UUID `json:"package_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type CompanionBundle struct {
	Intent            SubmissionIntent   `json:"intent"`
	Package           ApplicationPackage `json:"package"`
	ResumeContent     json.RawMessage     `json:"resume_content"`
	ResumePDFAvailable bool               `json:"resume_pdf_available"`
	ResumeFilename    string              `json:"resume_filename"`
}

// CreateHandoff mints a high-entropy capability for one PENDING intent. Only
// the SHA-256 digest is stored, so a database read cannot recover the bearer.
func (s *CompanionService) CreateHandoff(ctx context.Context, userID, intentID uuid.UUID) (CompanionHandoff, error) {
	intent, err := s.applications.GetSubmissionIntent(ctx, userID, intentID)
	if err != nil {
		return CompanionHandoff{}, err
	}
	if intent.Status != SubmissionPending {
		return CompanionHandoff{}, ErrStaleSubmissionLease
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return CompanionHandoff{}, err
	}
	token := hex.EncodeToString(raw)
	tokenHash := sha256Hex([]byte(token))
	expiresAt := time.Now().UTC().Add(companionTokenLifetime)

	_, err = s.repo.q.CreateSubmissionCompanionToken(ctx, db.CreateSubmissionCompanionTokenParams{
		UserID: database.UUIDToPG(userID),
		IntentID: database.UUIDToPG(intentID),
		TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CompanionHandoff{}, ErrStaleSubmissionLease
		}
		return CompanionHandoff{}, err
	}
	return CompanionHandoff{IntentID: intentID, PackageID: intent.PackageID, Token: token, ExpiresAt: expiresAt}, nil
}

func (s *CompanionService) authorize(ctx context.Context, intentID uuid.UUID, token string) (uuid.UUID, string, error) {
	if token == "" {
		return uuid.Nil, "", ErrCompanionUnauthorized
	}
	tokenHash := sha256Hex([]byte(token))
	row, err := s.repo.q.GetActiveSubmissionCompanionToken(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, "", ErrCompanionUnauthorized
		}
		return uuid.Nil, "", err
	}
	if database.PGToUUID(row.IntentID) != intentID {
		return uuid.Nil, "", ErrCompanionUnauthorized
	}
	return database.PGToUUID(row.UserID), tokenHash, nil
}

func (s *CompanionService) Bundle(ctx context.Context, intentID uuid.UUID, token string) (CompanionBundle, error) {
	userID, _, err := s.authorize(ctx, intentID, token)
	if err != nil {
		return CompanionBundle{}, err
	}
	intent, err := s.applications.GetSubmissionIntent(ctx, userID, intentID)
	if err != nil {
		return CompanionBundle{}, err
	}
	pkg, err := s.applications.GetApplicationPackage(ctx, userID, intent.PackageID)
	if err != nil {
		return CompanionBundle{}, err
	}
	version, err := s.repo.q.GetResumeVersionForUser(ctx, db.GetResumeVersionForUserParams{
		ID: database.UUIDToPG(pkg.ResumeVersionID), UserID: database.UUIDToPG(userID),
	})
	if err != nil {
		return CompanionBundle{}, err
	}
	return CompanionBundle{
		Intent: intent,
		Package: pkg,
		ResumeContent: append(json.RawMessage(nil), version.ContentJson...),
		ResumePDFAvailable: version.PdfStorageKey.Valid && version.PdfStorageKey.String != "",
		ResumeFilename: "applyforge-resume.pdf",
	}, nil
}

func (s *CompanionService) ResumePDF(ctx context.Context, intentID uuid.UUID, token string) ([]byte, string, error) {
	userID, _, err := s.authorize(ctx, intentID, token)
	if err != nil {
		return nil, "", err
	}
	intent, err := s.applications.GetSubmissionIntent(ctx, userID, intentID)
	if err != nil {
		return nil, "", err
	}
	pkg, err := s.applications.GetApplicationPackage(ctx, userID, intent.PackageID)
	if err != nil {
		return nil, "", err
	}
	version, err := s.repo.q.GetResumeVersionForUser(ctx, db.GetResumeVersionForUserParams{
		ID: database.UUIDToPG(pkg.ResumeVersionID), UserID: database.UUIDToPG(userID),
	})
	if err != nil {
		return nil, "", err
	}
	if !version.PdfStorageKey.Valid || version.PdfStorageKey.String == "" {
		return nil, "", ErrCompanionResumeUnavailable
	}
	data, err := s.storage.Get(ctx, version.PdfStorageKey.String)
	if err != nil {
		return nil, "", err
	}
	return data, "applyforge-resume.pdf", nil
}

func (s *CompanionService) Claim(ctx context.Context, intentID uuid.UUID, token, workerID string) (SubmissionIntent, error) {
	userID, _, err := s.authorize(ctx, intentID, token)
	if err != nil {
		return SubmissionIntent{}, err
	}
	return s.applications.ClaimSubmissionIntentForUser(ctx, userID, intentID, workerID)
}

func (s *CompanionService) Begin(ctx context.Context, intentID uuid.UUID, token, workerID string, generation int64) (SubmissionIntent, error) {
	userID, _, err := s.authorize(ctx, intentID, token)
	if err != nil {
		return SubmissionIntent{}, err
	}
	return s.applications.BeginSubmissionForUser(ctx, userID, intentID, workerID, generation)
}

func (s *CompanionService) Confirm(ctx context.Context, intentID uuid.UUID, token, workerID string, generation int64, receipt json.RawMessage) (SubmissionIntent, error) {
	userID, tokenHash, err := s.authorize(ctx, intentID, token)
	if err != nil {
		return SubmissionIntent{}, err
	}
	intent, err := s.applications.ConfirmSubmissionForUser(ctx, userID, intentID, workerID, generation, receipt)
	if err != nil {
		return SubmissionIntent{}, err
	}
	_ = s.repo.q.RevokeSubmissionCompanionToken(ctx, tokenHash)
	return intent, nil
}

func (s *CompanionService) Uncertain(ctx context.Context, intentID uuid.UUID, token, workerID string, generation int64, message string) (SubmissionIntent, error) {
	userID, tokenHash, err := s.authorize(ctx, intentID, token)
	if err != nil {
		return SubmissionIntent{}, err
	}
	intent, err := s.applications.MarkSubmissionUncertainForUser(ctx, userID, intentID, workerID, generation, message)
	if err != nil {
		return SubmissionIntent{}, err
	}
	_ = s.repo.q.RevokeSubmissionCompanionToken(ctx, tokenHash)
	return intent, nil
}
