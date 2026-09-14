package applications

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

const (
	ReconcileApplied      = "APPLIED"
	ReconcileNotSubmitted = "NOT_SUBMITTED"
)

var ErrInvalidSubmissionReconciliation = errors.New("invalid submission reconciliation")

// ReconcileSubmission resolves only UNCERTAIN intents and requires an explicit
// user-selected real-world outcome. No automated worker may call this method as
// a substitute for an ATS receipt.
func (s *Service) ReconcileSubmission(ctx context.Context, userID, intentID uuid.UUID, outcome, note string) (SubmissionIntent, error) {
	note = strings.TrimSpace(note)
	if note == "" {
		if outcome == ReconcileApplied {
			note = "user verified the employer ATS accepted the application"
		} else {
			note = "user verified the employer ATS did not receive the application"
		}
	}

	var (
		row db.SubmissionIntent
		err error
	)
	switch outcome {
	case ReconcileApplied:
		row, err = s.repo.q.ReconcileSubmissionAsApplied(ctx, db.ReconcileSubmissionAsAppliedParams{
			ID: database.UUIDToPG(intentID), UserID: database.UUIDToPG(userID), Note: note,
		})
	case ReconcileNotSubmitted:
		row, err = s.repo.q.ReconcileSubmissionAsNotSubmitted(ctx, db.ReconcileSubmissionAsNotSubmittedParams{
			ID: database.UUIDToPG(intentID), UserID: database.UUIDToPG(userID), Note: note,
		})
	default:
		return SubmissionIntent{}, ErrInvalidSubmissionReconciliation
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubmissionIntent{}, ErrSubmissionIntentNotFound
		}
		return SubmissionIntent{}, err
	}
	return submissionIntentFromRow(row), nil
}
