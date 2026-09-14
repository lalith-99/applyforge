package applications

import (
	"context"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
)

// ListLatestSubmissionIntentsForUser returns at most one current execution
// state per tracked application. It is used by the applications UI to surface
// UNCERTAIN work prominently rather than silently retrying it.
func (s *Service) ListLatestSubmissionIntentsForUser(ctx context.Context, userID uuid.UUID) ([]SubmissionIntent, error) {
	rows, err := s.repo.q.ListLatestSubmissionIntentsForUser(ctx, database.UUIDToPG(userID))
	if err != nil {
		return nil, err
	}
	result := make([]SubmissionIntent, 0, len(rows))
	for _, row := range rows {
		result = append(result, submissionIntentFromRow(row))
	}
	return result, nil
}
