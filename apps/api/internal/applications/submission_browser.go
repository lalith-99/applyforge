package applications

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// ClaimSubmissionIntentForUser claims one known intent owned by the caller.
// This is the browser-companion entry point; unlike the background-worker claim,
// it can never select another user's pending work.
func (s *Service) ClaimSubmissionIntentForUser(ctx context.Context, userID, intentID uuid.UUID, workerID string) (SubmissionIntent, error) {
	if workerID == "" {
		return SubmissionIntent{}, ErrInvalidSubmissionWorkerID
	}
	row, err := s.repo.q.ClaimSubmissionIntentForUser(ctx, db.ClaimSubmissionIntentForUserParams{
		ID: database.UUIDToPG(intentID), UserID: database.UUIDToPG(userID), WorkerID: workerID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubmissionIntent{}, ErrStaleSubmissionLease
		}
		return SubmissionIntent{}, err
	}
	return submissionIntentFromRow(row), nil
}

// BeginSubmissionForUser verifies ownership before crossing the irreversible
// boundary. The existing fenced BeginSubmission then validates worker/generation.
func (s *Service) BeginSubmissionForUser(ctx context.Context, userID, intentID uuid.UUID, workerID string, generation int64) (SubmissionIntent, error) {
	if _, err := s.GetSubmissionIntent(ctx, userID, intentID); err != nil {
		return SubmissionIntent{}, err
	}
	return s.BeginSubmission(ctx, intentID, workerID, generation)
}

// ConfirmSubmissionForUser verifies ownership before recording an external
// receipt and atomically advancing the application to APPLIED.
func (s *Service) ConfirmSubmissionForUser(ctx context.Context, userID, intentID uuid.UUID, workerID string, generation int64, receipt json.RawMessage) (SubmissionIntent, error) {
	if _, err := s.GetSubmissionIntent(ctx, userID, intentID); err != nil {
		return SubmissionIntent{}, err
	}
	return s.ConfirmSubmission(ctx, intentID, workerID, generation, receipt)
}

// MarkSubmissionUncertainForUser is used after the irreversible boundary when
// the companion cannot prove whether the ATS accepted the submission.
func (s *Service) MarkSubmissionUncertainForUser(ctx context.Context, userID, intentID uuid.UUID, workerID string, generation int64, message string) (SubmissionIntent, error) {
	if _, err := s.GetSubmissionIntent(ctx, userID, intentID); err != nil {
		return SubmissionIntent{}, err
	}
	return s.MarkSubmissionUncertain(ctx, intentID, workerID, generation, message)
}
