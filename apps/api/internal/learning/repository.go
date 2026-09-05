// Package learning implements Quick Prep, Defend This Bullet, learning
// plans, Interview Readiness, and Make Me Qualified (see
// MASTER_REQUIREMENTS.md §31-§35, §33).
package learning

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// ErrNotFound is returned when a cached quick-prep module or learning plan doesn't exist.
var ErrNotFound = errors.New("not found")

// Repository provides access to quick_prep_modules / learning_plans caches.
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

func quickPrepFromRow(row db.QuickPrepModule) aiclient.QuickPrepModule {
	var questions []aiclient.InterviewQuestion
	_ = json.Unmarshal(row.InterviewQuestions, &questions)
	return aiclient.QuickPrepModule{
		Skill:                 row.NormalizedSkill,
		WhatItIs:              row.WhatItIs,
		WhyItMatters:          row.WhyItMatters,
		TransferableFrom:      row.TransferableFrom,
		CoreConcepts:          row.CoreConcepts,
		ScreeningPoints:       row.ScreeningPoints,
		InterviewQuestions:    questions,
		CommonMistakes:        row.CommonMistakes,
		ArchitectureQuestions: row.ArchitectureQuestions,
		ExampleCode:           database.TextOrNil(row.ExampleCode),
	}
}

// GetCachedQuickPrep returns a cached module for a normalized skill, if present.
func (r *Repository) GetCachedQuickPrep(ctx context.Context, normalizedSkill string) (aiclient.QuickPrepModule, error) {
	row, err := r.q.GetQuickPrepModule(ctx, normalizedSkill)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return aiclient.QuickPrepModule{}, ErrNotFound
		}
		return aiclient.QuickPrepModule{}, err
	}
	return quickPrepFromRow(row), nil
}

// SaveQuickPrep caches a generated module for a normalized skill.
func (r *Repository) SaveQuickPrep(ctx context.Context, normalizedSkill string, module aiclient.QuickPrepModule) error {
	questionsJSON, _ := json.Marshal(module.InterviewQuestions)
	_, err := r.q.UpsertQuickPrepModule(ctx, db.UpsertQuickPrepModuleParams{
		NormalizedSkill:       normalizedSkill,
		WhatItIs:              module.WhatItIs,
		WhyItMatters:          module.WhyItMatters,
		TransferableFrom:      orEmpty(module.TransferableFrom),
		CoreConcepts:          orEmpty(module.CoreConcepts),
		ScreeningPoints:       orEmpty(module.ScreeningPoints),
		InterviewQuestions:    questionsJSON,
		CommonMistakes:        orEmpty(module.CommonMistakes),
		ArchitectureQuestions: orEmpty(module.ArchitectureQuestions),
		ExampleCode:           database.PGText(module.ExampleCode),
	})
	return err
}

// PlanResult is the domain representation of a stored learning plan.
type PlanResult struct {
	JobID                   uuid.UUID
	Skills                  []string
	CurrentReadiness        int32
	TargetReadiness         int32
	Topics                  []string
	PracticeQuestions       []aiclient.InterviewQuestion
	Projects                []string
	ArchitectureQuestions   []string
	EstimatedEffortCategory string
}

// SaveLearningPlan caches a generated learning plan for a (user, job) pair.
func (r *Repository) SaveLearningPlan(ctx context.Context, userID, jobID uuid.UUID, result aiclient.LearningPlanResult) (PlanResult, error) {
	topicsJSON, _ := json.Marshal(orEmpty(result.Topics))
	practiceJSON, _ := json.Marshal(result.PracticeQuestions)

	row, err := r.q.UpsertLearningPlan(ctx, db.UpsertLearningPlanParams{
		UserID:                  database.UUIDToPG(userID),
		JobID:                   database.UUIDToPG(jobID),
		Skills:                  orEmpty(result.Skills),
		CurrentReadiness:        int32(result.CurrentReadiness),
		TargetReadiness:         int32(result.TargetReadiness),
		Topics:                  topicsJSON,
		PracticeQuestions:       practiceJSON,
		Projects:                orEmpty(result.Projects),
		ArchitectureQuestions:   orEmpty(result.ArchitectureQuestions),
		EstimatedEffortCategory: result.EstimatedEffortCategory,
	})
	if err != nil {
		return PlanResult{}, err
	}

	var practiceQuestions []aiclient.InterviewQuestion
	_ = json.Unmarshal(row.PracticeQuestions, &practiceQuestions)

	return PlanResult{
		JobID:                   database.PGToUUID(row.JobID),
		Skills:                  row.Skills,
		CurrentReadiness:        row.CurrentReadiness,
		TargetReadiness:         row.TargetReadiness,
		Topics:                  decodeStrings(row.Topics),
		PracticeQuestions:       practiceQuestions,
		Projects:                row.Projects,
		ArchitectureQuestions:   row.ArchitectureQuestions,
		EstimatedEffortCategory: row.EstimatedEffortCategory,
	}, nil
}

func decodeStrings(raw []byte) []string {
	var out []string
	_ = json.Unmarshal(raw, &out)
	return out
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func normalizedKey(skill string) string {
	return strings.ToLower(strings.TrimSpace(skill))
}
