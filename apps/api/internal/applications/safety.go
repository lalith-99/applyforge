package applications

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// ErrResumeVersionMismatch is returned when a caller tries to attach a resume
// version that is not owned by the user or was not generated for the same job.
// Application execution must never be able to substitute a cross-user or
// cross-job artifact.
var ErrResumeVersionMismatch = errors.New("resume version does not belong to user and job")

// ValidateResumeVersionForJob verifies that a supplied resume version is both
// owned by the caller and explicitly scoped to the job being saved.
func (r *Repository) ValidateResumeVersionForJob(ctx context.Context, userID, jobID uuid.UUID, resumeVersionID *uuid.UUID) error {
	if resumeVersionID == nil {
		return nil
	}

	version, err := r.q.GetResumeVersionForUser(ctx, db.GetResumeVersionForUserParams{
		ID:     database.UUIDToPG(*resumeVersionID),
		UserID: database.UUIDToPG(userID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrResumeVersionMismatch
		}
		return err
	}
	if !version.JobID.Valid || database.PGToUUID(version.JobID) != jobID {
		return ErrResumeVersionMismatch
	}
	return nil
}

// ChangeStatusWithEvent performs the status mutation and its audit-event write
// in one PostgreSQL statement. The row is locked first so concurrent transitions
// record the actual prior status and cannot leave status/history out of sync.
func (r *Repository) ChangeStatusWithEvent(ctx context.Context, applicationID, userID uuid.UUID, status string, notes *string) (Application, error) {
	row, err := r.q.ChangeApplicationStatusWithEvent(ctx, db.ChangeApplicationStatusWithEventParams{
		ID:     database.UUIDToPG(applicationID),
		UserID: database.UUIDToPG(userID),
		Status: status,
		Notes:  database.PGText(notes),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Application{}, ErrNotFound
		}
		return Application{}, err
	}
	return fromRow(row), nil
}
