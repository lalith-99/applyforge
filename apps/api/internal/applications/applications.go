// Package applications tracks a candidate's job applications end to end:
// status (Kanban stages), status-change history, and reusable application
// answers (see MASTER_REQUIREMENTS.md §39-§41).
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

// ErrNotFound is returned when an application doesn't exist (or isn't owned by the caller).
var ErrNotFound = errors.New("application not found")

// Status values (mirrors the applications.status check constraint), in
// typical Kanban column order.
const (
	StatusSaved              = "SAVED"
	StatusReadyToApply       = "READY_TO_APPLY"
	StatusApplied            = "APPLIED"
	StatusRecruiterScreen    = "RECRUITER_SCREEN"
	StatusAssessment         = "ASSESSMENT"
	StatusTechnicalInterview = "TECHNICAL_INTERVIEW"
	StatusFinalInterview     = "FINAL_INTERVIEW"
	StatusOffer              = "OFFER"
	StatusRejected           = "REJECTED"
	StatusWithdrawn          = "WITHDRAWN"
)

// ValidStatuses lists every allowed status value, for request validation.
var ValidStatuses = map[string]bool{
	StatusSaved:              true,
	StatusReadyToApply:       true,
	StatusApplied:            true,
	StatusRecruiterScreen:    true,
	StatusAssessment:         true,
	StatusTechnicalInterview: true,
	StatusFinalInterview:     true,
	StatusOffer:              true,
	StatusRejected:           true,
	StatusWithdrawn:          true,
}

// Application is the domain representation of a tracked job application.
type Application struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	JobID           uuid.UUID
	ResumeVersionID *uuid.UUID
	Status          string
	MatchScore      *int32
	Notes           *string
	NextAction      *string
	AppliedAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func fromRow(row db.Application) Application {
	return Application{
		ID:              database.PGToUUID(row.ID),
		UserID:          database.PGToUUID(row.UserID),
		JobID:           database.PGToUUID(row.JobID),
		ResumeVersionID: database.UUIDOrNil(row.ResumeVersionID),
		Status:          row.Status,
		MatchScore:      database.Int4OrNil(row.MatchScore),
		Notes:           database.TextOrNil(row.Notes),
		NextAction:      database.TextOrNil(row.NextAction),
		AppliedAt:       database.TimeOrNil(row.AppliedAt),
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
	}
}

// WithJob is an Application joined with the job's display fields, for the
// applications list/Kanban/table UI.
type WithJob struct {
	Application
	CompanyName     string
	Title           string
	NormalizedTitle string
	Source          string
	FirstSeenAt     time.Time
	PostedAt        *time.Time
}

func withJobFromRow(row db.ListApplicationsWithJobForUserRow) WithJob {
	return WithJob{
		Application: Application{
			ID:              database.PGToUUID(row.ID),
			UserID:          database.PGToUUID(row.UserID),
			JobID:           database.PGToUUID(row.JobID),
			ResumeVersionID: database.UUIDOrNil(row.ResumeVersionID),
			Status:          row.Status,
			MatchScore:      database.Int4OrNil(row.MatchScore),
			Notes:           database.TextOrNil(row.Notes),
			NextAction:      database.TextOrNil(row.NextAction),
			AppliedAt:       database.TimeOrNil(row.AppliedAt),
			CreatedAt:       row.CreatedAt.Time,
			UpdatedAt:       row.UpdatedAt.Time,
		},
		CompanyName:     row.CompanyName,
		Title:           row.Title,
		NormalizedTitle: row.NormalizedTitle,
		Source:          row.Source,
		FirstSeenAt:     row.FirstSeenAt.Time,
		PostedAt:        database.TimeOrNil(row.PostedAt),
	}
}

// Event is a single status-change (or other) event in an application's history.
type Event struct {
	ID            uuid.UUID
	ApplicationID uuid.UUID
	EventType     string
	FromStatus    *string
	ToStatus      *string
	Notes         *string
	CreatedAt     time.Time
}

func eventFromRow(row db.ApplicationEvent) Event {
	return Event{
		ID:            database.PGToUUID(row.ID),
		ApplicationID: database.PGToUUID(row.ApplicationID),
		EventType:     row.EventType,
		FromStatus:    database.TextOrNil(row.FromStatus),
		ToStatus:      database.TextOrNil(row.ToStatus),
		Notes:         database.TextOrNil(row.Notes),
		CreatedAt:     row.CreatedAt.Time,
	}
}

// Answers holds a user's reusable application-form answers (see §41).
type Answers struct {
	UserID            uuid.UUID
	FullName          *string
	Phone             *string
	Email             *string
	Location          *string
	DesiredLocation   *string
	WorkAuthorization *string
	Sponsorship       *string
	SalaryExpectation *string
	NoticePeriod      *string
	LinkedinURL       *string
	GithubURL         *string
	PortfolioURL      *string
	CommonAnswers     json.RawMessage
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func answersFromRow(row db.ApplicationAnswer) Answers {
	return Answers{
		UserID:            database.PGToUUID(row.UserID),
		FullName:          database.TextOrNil(row.FullName),
		Phone:             database.TextOrNil(row.Phone),
		Email:             database.TextOrNil(row.Email),
		Location:          database.TextOrNil(row.Location),
		DesiredLocation:   database.TextOrNil(row.DesiredLocation),
		WorkAuthorization: database.TextOrNil(row.WorkAuthorization),
		Sponsorship:       database.TextOrNil(row.Sponsorship),
		SalaryExpectation: database.TextOrNil(row.SalaryExpectation),
		NoticePeriod:      database.TextOrNil(row.NoticePeriod),
		LinkedinURL:       database.TextOrNil(row.LinkedinUrl),
		GithubURL:         database.TextOrNil(row.GithubUrl),
		PortfolioURL:      database.TextOrNil(row.PortfolioUrl),
		CommonAnswers:     row.CommonAnswers,
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
	}
}

// Repository provides access to applications / application_events / application_answers records.
type Repository struct {
	q *db.Queries
}

// NewRepository builds a Repository from a database pool.
func NewRepository(pool *database.Pool) *Repository {
	return &Repository{q: pool.Queries()}
}

// NewRepositoryFromQueries builds a Repository directly from generated
// Queries — used by tests that run against a transaction-scoped connection.
func NewRepositoryFromQueries(q *db.Queries) *Repository {
	return &Repository{q: q}
}

// Create inserts a new application, or (if one already exists for this
// user+job) attaches the given resume version without resetting status.
func (r *Repository) Create(ctx context.Context, userID, jobID uuid.UUID, resumeVersionID *uuid.UUID, status string, matchScore *int32) (Application, error) {
	row, err := r.q.CreateApplication(ctx, db.CreateApplicationParams{
		UserID:          database.UUIDToPG(userID),
		JobID:           database.UUIDToPG(jobID),
		ResumeVersionID: database.PGUUID(resumeVersionID),
		Status:          status,
		MatchScore:      database.PGInt4(matchScore),
	})
	if err != nil {
		return Application{}, err
	}
	return fromRow(row), nil
}

// GetForUser returns an application owned by the given user.
func (r *Repository) GetForUser(ctx context.Context, id, userID uuid.UUID) (Application, error) {
	row, err := r.q.GetApplicationForUser(ctx, db.GetApplicationForUserParams{
		ID:     database.UUIDToPG(id),
		UserID: database.UUIDToPG(userID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Application{}, ErrNotFound
		}
		return Application{}, err
	}
	return fromRow(row), nil
}

// ListForUser returns all of a user's applications, most recently updated first.
func (r *Repository) ListForUser(ctx context.Context, userID uuid.UUID) ([]Application, error) {
	rows, err := r.q.ListApplicationsForUser(ctx, database.UUIDToPG(userID))
	if err != nil {
		return nil, err
	}
	out := make([]Application, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromRow(row))
	}
	return out, nil
}

// ListWithJobForUser returns all of a user's applications joined with job display fields.
func (r *Repository) ListWithJobForUser(ctx context.Context, userID uuid.UUID) ([]WithJob, error) {
	rows, err := r.q.ListApplicationsWithJobForUser(ctx, database.UUIDToPG(userID))
	if err != nil {
		return nil, err
	}
	out := make([]WithJob, 0, len(rows))
	for _, row := range rows {
		out = append(out, withJobFromRow(row))
	}
	return out, nil
}

// UpdateStatus transitions an application to a new status (setting
// applied_at automatically on first transition to APPLIED).
func (r *Repository) UpdateStatus(ctx context.Context, id, userID uuid.UUID, status string) (Application, error) {
	row, err := r.q.UpdateApplicationStatus(ctx, db.UpdateApplicationStatusParams{
		ID:     database.UUIDToPG(id),
		UserID: database.UUIDToPG(userID),
		Status: status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Application{}, ErrNotFound
		}
		return Application{}, err
	}
	return fromRow(row), nil
}

// UpdateNotes updates the free-text notes and next-action fields.
func (r *Repository) UpdateNotes(ctx context.Context, id, userID uuid.UUID, notes, nextAction *string) (Application, error) {
	row, err := r.q.UpdateApplicationNotes(ctx, db.UpdateApplicationNotesParams{
		ID:         database.UUIDToPG(id),
		UserID:     database.UUIDToPG(userID),
		Notes:      database.PGText(notes),
		NextAction: database.PGText(nextAction),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Application{}, ErrNotFound
		}
		return Application{}, err
	}
	return fromRow(row), nil
}

// CreateEvent records a status-change (or other) event in an application's history.
func (r *Repository) CreateEvent(ctx context.Context, applicationID uuid.UUID, eventType string, fromStatus, toStatus, notes *string) (Event, error) {
	row, err := r.q.CreateApplicationEvent(ctx, db.CreateApplicationEventParams{
		ApplicationID: database.UUIDToPG(applicationID),
		EventType:     eventType,
		FromStatus:    database.PGText(fromStatus),
		ToStatus:      database.PGText(toStatus),
		Notes:         database.PGText(notes),
	})
	if err != nil {
		return Event{}, err
	}
	return eventFromRow(row), nil
}

// ListEvents returns an application's event history, oldest first.
func (r *Repository) ListEvents(ctx context.Context, applicationID uuid.UUID) ([]Event, error) {
	rows, err := r.q.ListApplicationEvents(ctx, database.UUIDToPG(applicationID))
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(rows))
	for _, row := range rows {
		out = append(out, eventFromRow(row))
	}
	return out, nil
}

// GetAnswers returns a user's saved application answers.
func (r *Repository) GetAnswers(ctx context.Context, userID uuid.UUID) (Answers, error) {
	row, err := r.q.GetApplicationAnswers(ctx, database.UUIDToPG(userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Answers{}, ErrNotFound
		}
		return Answers{}, err
	}
	return answersFromRow(row), nil
}

// UpsertAnswersInput carries the fields accepted by UpsertAnswers.
type UpsertAnswersInput struct {
	FullName          *string         `json:"full_name"`
	Phone             *string         `json:"phone"`
	Email             *string         `json:"email"`
	Location          *string         `json:"location"`
	DesiredLocation   *string         `json:"desired_location"`
	WorkAuthorization *string         `json:"work_authorization"`
	Sponsorship       *string         `json:"sponsorship"`
	SalaryExpectation *string         `json:"salary_expectation"`
	NoticePeriod      *string         `json:"notice_period"`
	LinkedinURL       *string         `json:"linkedin_url"`
	GithubURL         *string         `json:"github_url"`
	PortfolioURL      *string         `json:"portfolio_url"`
	CommonAnswers     json.RawMessage `json:"common_answers"`
}

// UpsertAnswers creates or replaces a user's saved application answers.
func (r *Repository) UpsertAnswers(ctx context.Context, userID uuid.UUID, in UpsertAnswersInput) (Answers, error) {
	commonAnswers := in.CommonAnswers
	if commonAnswers == nil {
		commonAnswers = json.RawMessage(`{}`)
	}
	row, err := r.q.UpsertApplicationAnswers(ctx, db.UpsertApplicationAnswersParams{
		UserID:            database.UUIDToPG(userID),
		FullName:          database.PGText(in.FullName),
		Phone:             database.PGText(in.Phone),
		Email:             database.PGText(in.Email),
		Location:          database.PGText(in.Location),
		DesiredLocation:   database.PGText(in.DesiredLocation),
		WorkAuthorization: database.PGText(in.WorkAuthorization),
		Sponsorship:       database.PGText(in.Sponsorship),
		SalaryExpectation: database.PGText(in.SalaryExpectation),
		NoticePeriod:      database.PGText(in.NoticePeriod),
		LinkedinUrl:       database.PGText(in.LinkedinURL),
		GithubUrl:         database.PGText(in.GithubURL),
		PortfolioUrl:      database.PGText(in.PortfolioURL),
		CommonAnswers:     commonAnswers,
	})
	if err != nil {
		return Answers{}, err
	}
	return answersFromRow(row), nil
}
