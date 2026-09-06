// Package tailoring implements AI-assisted resume tailoring: STRICT/GROWTH/
// MAX_MATCH suggestion generation, user approval workflow, and the Resume
// Alignment Score (see MASTER_REQUIREMENTS.md §23, §26-§30).
package tailoring

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

// ErrNotFound is returned when a tailoring run or suggestion doesn't exist
// (or isn't owned by the caller).
var ErrNotFound = errors.New("tailoring run not found")

// Modes (mirrors the tailoring_runs.mode check constraint).
const (
	ModeStrict   = "STRICT"
	ModeGrowth   = "GROWTH"
	ModeMaxMatch = "MAX_MATCH"
)

// Suggestion statuses (mirrors the tailoring_suggestions.user_status check constraint).
const (
	StatusPending  = "PENDING"
	StatusApproved = "APPROVED"
	StatusEdited   = "EDITED"
	StatusRejected = "REJECTED"
)

// Run statuses (mirrors the tailoring_runs.status check constraint). A run
// starts PENDING (queued for a background worker - see Phase J/K), then
// moves through WRITING/EVALUATING/REVISING before a terminal COMPLETED or
// FAILED, so a polling UI can show real progress for what may now take a
// couple of AI calls.
const (
	RunStatusPending    = "PENDING"
	RunStatusWriting    = "WRITING"
	RunStatusEvaluating = "EVALUATING"
	RunStatusRevising   = "REVISING"
	RunStatusCompleted  = "COMPLETED"
	RunStatusFailed     = "FAILED"
)

// Run is the domain representation of a tailoring run.
type Run struct {
	ID                   uuid.UUID
	UserID               uuid.UUID
	JobID                uuid.UUID
	ResumeID             uuid.UUID
	Mode                 string
	Status               string
	SummarySuggestion    json.RawMessage
	AlignmentScoreBefore *int32
	AlignmentScoreAfter  *int32
	CriticResult         json.RawMessage
	RevisionCount        int32
	CreatedAt            time.Time
	CompletedAt          *time.Time
}

func runFromRow(row db.TailoringRun) Run {
	return Run{
		ID:                   database.PGToUUID(row.ID),
		UserID:               database.PGToUUID(row.UserID),
		JobID:                database.PGToUUID(row.JobID),
		ResumeID:             database.PGToUUID(row.ResumeID),
		Mode:                 row.Mode,
		Status:               row.Status,
		SummarySuggestion:    row.SummarySuggestion,
		AlignmentScoreBefore: database.Int4OrNil(row.AlignmentScoreBefore),
		AlignmentScoreAfter:  database.Int4OrNil(row.AlignmentScoreAfter),
		CriticResult:         row.CriticResult,
		RevisionCount:        row.RevisionCount,
		CreatedAt:            row.CreatedAt.Time,
		CompletedAt:          database.TimeOrNil(row.CompletedAt),
	}
}

// Suggestion is a single proposed resume change.
type Suggestion struct {
	ID                    uuid.UUID
	TailoringRunID        uuid.UUID
	Section               string
	OriginalText          *string
	SuggestedText         string
	RequirementsAddressed []string
	SkillsAdded           []string
	KeywordsAdded         []string
	Source                string
	Reason                string
	Confidence            float64
	RiskLevel             string
	UserStatus            string
	EditedText            *string
}

func suggestionFromRow(row db.TailoringSuggestion) Suggestion {
	return Suggestion{
		ID:                    database.PGToUUID(row.ID),
		TailoringRunID:        database.PGToUUID(row.TailoringRunID),
		Section:               row.Section,
		OriginalText:          database.TextOrNil(row.OriginalText),
		SuggestedText:         row.SuggestedText,
		RequirementsAddressed: row.RequirementsAddressed,
		SkillsAdded:           row.SkillsAdded,
		KeywordsAdded:         row.KeywordsAdded,
		Source:                row.Source,
		Reason:                row.Reason,
		Confidence:            row.Confidence,
		RiskLevel:             row.RiskLevel,
		UserStatus:            row.UserStatus,
		EditedText:            database.TextOrNil(row.EditedText),
	}
}

// Repository provides access to tailoring_runs / tailoring_suggestions records.
type Repository struct {
	q *db.Queries
}

// NewRepository builds a Repository from a database pool.
func NewRepository(pool *database.Pool) *Repository {
	return &Repository{q: pool.Queries()}
}

// CreateRun starts a new tailoring run.
func (r *Repository) CreateRun(ctx context.Context, userID, jobID, resumeID uuid.UUID, mode string, alignmentBefore int32) (Run, error) {
	row, err := r.q.CreateTailoringRun(ctx, db.CreateTailoringRunParams{
		UserID:               database.UUIDToPG(userID),
		JobID:                database.UUIDToPG(jobID),
		ResumeID:             database.UUIDToPG(resumeID),
		Mode:                 mode,
		AlignmentScoreBefore: database.PGInt4(&alignmentBefore),
	})
	if err != nil {
		return Run{}, err
	}
	return runFromRow(row), nil
}

// CompleteRun marks a run completed and stores the summary suggestion + coverage + after-score.
func (r *Repository) CompleteRun(ctx context.Context, runID uuid.UUID, summarySuggestion json.RawMessage, keywordCoverage json.RawMessage, alignmentAfter int32) (Run, error) {
	row, err := r.q.CompleteTailoringRun(ctx, db.CompleteTailoringRunParams{
		ID:                  database.UUIDToPG(runID),
		SummarySuggestion:   summarySuggestion,
		KeywordCoverage:     keywordCoverage,
		AlignmentScoreAfter: database.PGInt4(&alignmentAfter),
	})
	if err != nil {
		return Run{}, err
	}
	return runFromRow(row), nil
}

// FailRun marks a run failed.
func (r *Repository) FailRun(ctx context.Context, runID uuid.UUID) error {
	return r.q.FailTailoringRun(ctx, database.UUIDToPG(runID))
}

// UpdateStatus advances a run through an intermediate stage (WRITING/
// EVALUATING/REVISING) for a polling UI.
func (r *Repository) UpdateStatus(ctx context.Context, runID uuid.UUID, status string) error {
	return r.q.UpdateTailoringRunStatus(ctx, db.UpdateTailoringRunStatusParams{
		ID:     database.UUIDToPG(runID),
		Status: status,
	})
}

// SetCritic stores the AI critic's evaluation and how many revision passes
// have run so far (Phase K).
func (r *Repository) SetCritic(ctx context.Context, runID uuid.UUID, criticResult json.RawMessage, revisionCount int32) error {
	return r.q.SetTailoringRunCritic(ctx, db.SetTailoringRunCriticParams{
		ID:            database.UUIDToPG(runID),
		CriticResult:  criticResult,
		RevisionCount: revisionCount,
	})
}

// GetRunForUser returns a tailoring run owned by the given user.
func (r *Repository) GetRunForUser(ctx context.Context, runID, userID uuid.UUID) (Run, error) {
	row, err := r.q.GetTailoringRunForUser(ctx, db.GetTailoringRunForUserParams{ID: database.UUIDToPG(runID), UserID: database.UUIDToPG(userID)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Run{}, ErrNotFound
		}
		return Run{}, err
	}
	return runFromRow(row), nil
}

// GetRun returns a tailoring run regardless of owner (used by the async
// background worker, which is dispatched by run ID rather than a request
// scoped to a specific user).
func (r *Repository) GetRun(ctx context.Context, runID uuid.UUID) (Run, error) {
	row, err := r.q.GetTailoringRun(ctx, database.UUIDToPG(runID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Run{}, ErrNotFound
		}
		return Run{}, err
	}
	return runFromRow(row), nil
}

// AddSuggestion appends a suggestion to a run.
func (r *Repository) AddSuggestion(ctx context.Context, runID uuid.UUID, s Suggestion) (Suggestion, error) {
	row, err := r.q.CreateTailoringSuggestion(ctx, db.CreateTailoringSuggestionParams{
		TailoringRunID:        database.UUIDToPG(runID),
		Section:               s.Section,
		OriginalText:          database.PGText(s.OriginalText),
		SuggestedText:         s.SuggestedText,
		RequirementsAddressed: orEmpty(s.RequirementsAddressed),
		SkillsAdded:           orEmpty(s.SkillsAdded),
		KeywordsAdded:         orEmpty(s.KeywordsAdded),
		Source:                s.Source,
		Reason:                s.Reason,
		Confidence:            s.Confidence,
		RiskLevel:             s.RiskLevel,
	})
	if err != nil {
		return Suggestion{}, err
	}
	return suggestionFromRow(row), nil
}

// ListSuggestions returns all suggestions for a run, in creation order.
func (r *Repository) ListSuggestions(ctx context.Context, runID uuid.UUID) ([]Suggestion, error) {
	rows, err := r.q.ListTailoringSuggestions(ctx, database.UUIDToPG(runID))
	if err != nil {
		return nil, err
	}
	out := make([]Suggestion, 0, len(rows))
	for _, row := range rows {
		out = append(out, suggestionFromRow(row))
	}
	return out, nil
}

// UpdateSuggestionStatus approves/edits/rejects a single suggestion.
func (r *Repository) UpdateSuggestionStatus(ctx context.Context, suggestionID, runID uuid.UUID, status string, editedText *string) (Suggestion, error) {
	row, err := r.q.UpdateTailoringSuggestionStatus(ctx, db.UpdateTailoringSuggestionStatusParams{
		ID:             database.UUIDToPG(suggestionID),
		TailoringRunID: database.UUIDToPG(runID),
		UserStatus:     status,
		EditedText:     database.PGText(editedText),
	})
	if err != nil {
		return Suggestion{}, err
	}
	return suggestionFromRow(row), nil
}

// ApproveAllPending flips every PENDING suggestion in a run to APPROVED.
func (r *Repository) ApproveAllPending(ctx context.Context, runID uuid.UUID) error {
	return r.q.ApproveAllPendingSuggestions(ctx, database.UUIDToPG(runID))
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
