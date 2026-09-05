package applications

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrInvalidStatus is returned when a caller requests a transition to an unknown status.
var ErrInvalidStatus = errors.New("invalid application status")

// Service orchestrates application status transitions, logging an event for
// every change so the full history is auditable (see §40).
type Service struct {
	repo *Repository
}

// NewService builds a Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Save creates (or re-attaches a resume version to) a tracked application
// for a job, defaulting to the SAVED status. CreateApplication is an upsert
// (unique on user_id+job_id) so calling this again just updates the resume
// version link without resetting an in-progress application's status.
func (s *Service) Save(ctx context.Context, userID, jobID uuid.UUID, resumeVersionID *uuid.UUID, matchScore *int32) (Application, error) {
	return s.repo.Create(ctx, userID, jobID, resumeVersionID, StatusSaved, matchScore)
}

// ChangeStatus transitions an application to a new status and records the
// transition as an event.
func (s *Service) ChangeStatus(ctx context.Context, userID, applicationID uuid.UUID, newStatus string, notes *string) (Application, error) {
	if !ValidStatuses[newStatus] {
		return Application{}, ErrInvalidStatus
	}

	current, err := s.repo.GetForUser(ctx, applicationID, userID)
	if err != nil {
		return Application{}, err
	}

	updated, err := s.repo.UpdateStatus(ctx, applicationID, userID, newStatus)
	if err != nil {
		return Application{}, err
	}

	if current.Status != newStatus {
		fromStatus := current.Status
		if _, err := s.repo.CreateEvent(ctx, applicationID, "STATUS_CHANGE", &fromStatus, &newStatus, notes); err != nil {
			return Application{}, err
		}
	}

	return updated, nil
}
