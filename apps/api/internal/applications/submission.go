package applications

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

const (
	SubmissionPending    = "PENDING"
	SubmissionClaimed    = "CLAIMED"
	SubmissionSubmitting = "SUBMITTING"
	SubmissionConfirmed  = "CONFIRMED"
	SubmissionUncertain  = "UNCERTAIN"
	SubmissionFailed     = "FAILED"
	SubmissionCancelled  = "CANCELLED"
)

var (
	ErrActiveApprovalRequired    = errors.New("active application approval required")
	ErrSubmissionIntentNotFound  = errors.New("submission intent not found")
	ErrStaleSubmissionLease      = errors.New("submission lease is stale or no longer authorized")
	ErrInvalidSubmissionWorkerID = errors.New("submission worker id is required")
)

// SubmissionIntent is the durable, idempotent authorization to attempt one
// real-world submission. LeaseGeneration is a fencing token: every state
// mutation after claim must present the generation that was actually claimed.
type SubmissionIntent struct {
	ID              uuid.UUID       `json:"id"`
	ApplicationID   uuid.UUID       `json:"application_id"`
	PackageID       uuid.UUID       `json:"package_id"`
	ApprovalID      uuid.UUID       `json:"approval_id"`
	PackageHash     string          `json:"package_hash"`
	IdempotencyKey  string          `json:"idempotency_key"`
	Status          string          `json:"status"`
	LeaseGeneration int64           `json:"lease_generation"`
	LeaseOwner      *string         `json:"lease_owner,omitempty"`
	LeaseExpiresAt  *time.Time      `json:"lease_expires_at,omitempty"`
	AttemptCount    int32           `json:"attempt_count"`
	LastError       *string         `json:"last_error,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
}

func submissionIntentFromRow(row db.SubmissionIntent) SubmissionIntent {
	return SubmissionIntent{
		ID:              database.PGToUUID(row.ID),
		ApplicationID:   database.PGToUUID(row.ApplicationID),
		PackageID:       database.PGToUUID(row.PackageID),
		ApprovalID:      database.PGToUUID(row.ApprovalID),
		PackageHash:     row.PackageHash,
		IdempotencyKey:  row.IdempotencyKey,
		Status:          row.Status,
		LeaseGeneration: row.LeaseGeneration,
		LeaseOwner:      database.TextOrNil(row.LeaseOwner),
		LeaseExpiresAt:  database.TimeOrNil(row.LeaseExpiresAt),
		AttemptCount:    row.AttemptCount,
		LastError:       database.TextOrNil(row.LastError),
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
		CompletedAt:     database.TimeOrNil(row.CompletedAt),
	}
}

// CreateSubmissionIntent is idempotent for a package. The INSERT itself proves
// that the package still has a live, unrevoked SUBMIT_ONCE approval with an
// exactly matching package hash.
func (s *Service) CreateSubmissionIntent(ctx context.Context, userID, packageID uuid.UUID) (SubmissionIntent, error) {
	pkg, err := s.GetApplicationPackage(ctx, userID, packageID)
	if err != nil {
		return SubmissionIntent{}, err
	}
	app, err := s.repo.GetForUser(ctx, pkg.ApplicationID, userID)
	if err != nil {
		return SubmissionIntent{}, err
	}
	if app.Status != StatusReadyToApply {
		return SubmissionIntent{}, ErrApplicationNotReady
	}

	idempotencyKey := sha256Hex([]byte("submit-once-v1:" + pkg.PackageHash))
	row, err := s.repo.q.CreateSubmissionIntent(ctx, db.CreateSubmissionIntentParams{
		PackageID: database.UUIDToPG(packageID), UserID: database.UUIDToPG(userID), IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubmissionIntent{}, ErrActiveApprovalRequired
		}
		return SubmissionIntent{}, err
	}
	return submissionIntentFromRow(row), nil
}

func (s *Service) GetSubmissionIntent(ctx context.Context, userID, intentID uuid.UUID) (SubmissionIntent, error) {
	row, err := s.repo.q.GetSubmissionIntentForUser(ctx, db.GetSubmissionIntentForUserParams{
		ID: database.UUIDToPG(intentID), UserID: database.UUIDToPG(userID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubmissionIntent{}, ErrSubmissionIntentNotFound
		}
		return SubmissionIntent{}, err
	}
	return submissionIntentFromRow(row), nil
}

// ClaimNextSubmissionIntent gives one executor a five-minute lease and a new
// fencing generation. PENDING work with an expired/revoked approval is skipped;
// expired SUBMITTING work is moved to UNCERTAIN by the claim query instead of
// being retried automatically.
func (s *Service) ClaimNextSubmissionIntent(ctx context.Context, workerID string) (SubmissionIntent, error) {
	if workerID == "" {
		return SubmissionIntent{}, ErrInvalidSubmissionWorkerID
	}
	row, err := s.repo.q.ClaimNextSubmissionIntent(ctx, workerID)
	if err != nil {
		return SubmissionIntent{}, err
	}
	return submissionIntentFromRow(row), nil
}

// BeginSubmission is the final database gate before an executor may perform an
// irreversible external action. The approval and the exact fencing generation
// must still be current.
func (s *Service) BeginSubmission(ctx context.Context, intentID uuid.UUID, workerID string, generation int64) (SubmissionIntent, error) {
	row, err := s.repo.q.BeginSubmission(ctx, db.BeginSubmissionParams{
		ID: database.UUIDToPG(intentID), WorkerID: workerID, LeaseGeneration: generation,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubmissionIntent{}, ErrStaleSubmissionLease
		}
		return SubmissionIntent{}, err
	}
	return submissionIntentFromRow(row), nil
}

// ConfirmSubmission records the receipt, marks the intent CONFIRMED, and marks
// the tracked application APPLIED in the same PostgreSQL statement.
func (s *Service) ConfirmSubmission(ctx context.Context, intentID uuid.UUID, workerID string, generation int64, receipt json.RawMessage) (SubmissionIntent, error) {
	receipt = canonicalJSON(receipt)
	row, err := s.repo.q.ConfirmSubmission(ctx, db.ConfirmSubmissionParams{
		ID: database.UUIDToPG(intentID), WorkerID: workerID, LeaseGeneration: generation, ReceiptJson: receipt,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubmissionIntent{}, ErrStaleSubmissionLease
		}
		return SubmissionIntent{}, err
	}
	return submissionIntentFromRow(row), nil
}

// MarkSubmissionUncertain is for the dangerous case where an executor crossed
// the irreversible boundary but cannot prove whether the external system
// accepted the application. UNCERTAIN is terminal for automatic retry.
func (s *Service) MarkSubmissionUncertain(ctx context.Context, intentID uuid.UUID, workerID string, generation int64, message string) (SubmissionIntent, error) {
	row, err := s.repo.q.MarkSubmissionUncertain(ctx, db.MarkSubmissionUncertainParams{
		ID: database.UUIDToPG(intentID), WorkerID: workerID, LeaseGeneration: generation, ErrorMessage: message,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubmissionIntent{}, ErrStaleSubmissionLease
		}
		return SubmissionIntent{}, err
	}
	return submissionIntentFromRow(row), nil
}
