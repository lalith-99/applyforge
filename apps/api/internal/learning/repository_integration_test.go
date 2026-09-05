package learning_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobs"
	"github.com/lalithlochan/applyforge/apps/api/internal/learning"
	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
	"github.com/lalithlochan/applyforge/apps/api/internal/users"
)

func TestRepository_QuickPrepCacheMissThenHit(t *testing.T) {
	q := testdb.OpenTx(t)
	repo := learning.NewRepositoryFromQueries(q)
	ctx := context.Background()

	skill := fmt.Sprintf("test-skill-%s", uuid.NewString())

	if _, err := repo.GetCachedQuickPrep(ctx, skill); err != learning.ErrNotFound {
		t.Fatalf("expected ErrNotFound before first save, got %v", err)
	}

	example := "example code"
	module := aiclient.QuickPrepModule{
		WhatItIs:              "It is a thing.",
		WhyItMatters:          "It matters.",
		CoreConcepts:          []string{"concept a"},
		ScreeningPoints:       []string{"point a"},
		InterviewQuestions:    []aiclient.InterviewQuestion{{Question: "Q?", ConciseAnswer: "A.", DeeperExplanation: "Because."}},
		CommonMistakes:        []string{"mistake a"},
		ArchitectureQuestions: []string{"arch question a"},
		ExampleCode:           &example,
	}

	if err := repo.SaveQuickPrep(ctx, skill, module); err != nil {
		t.Fatalf("SaveQuickPrep: %v", err)
	}

	fetched, err := repo.GetCachedQuickPrep(ctx, skill)
	if err != nil {
		t.Fatalf("GetCachedQuickPrep: %v", err)
	}
	if fetched.WhatItIs != module.WhatItIs {
		t.Fatalf("expected what_it_is to round-trip, got %q", fetched.WhatItIs)
	}
	if len(fetched.InterviewQuestions) != 1 || fetched.InterviewQuestions[0].Question != "Q?" {
		t.Fatalf("expected interview questions to round-trip, got %+v", fetched.InterviewQuestions)
	}
	if fetched.ExampleCode == nil || *fetched.ExampleCode != example {
		t.Fatalf("expected example_code to round-trip")
	}
}

func TestRepository_SaveAndUpsertLearningPlan(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	jobsRepo := jobs.NewRepositoryFromQueries(q)
	repo := learning.NewRepositoryFromQueries(q)
	ctx := context.Background()

	email := fmt.Sprintf("learning-%s@example.com", uuid.NewString())
	user, err := userRepo.CreateWithPassword(ctx, email, "hash")
	if err != nil {
		t.Fatalf("create fixture user: %v", err)
	}

	externalID := uuid.NewString()
	companyID, err := jobsRepo.UpsertCompany(ctx, "Acme", fmt.Sprintf("acme-learning-%s", externalID))
	if err != nil {
		t.Fatalf("UpsertCompany: %v", err)
	}
	upserted, err := jobsRepo.UpsertJob(ctx, jobs.Job{
		Source:          "GREENHOUSE",
		ExternalID:      externalID,
		CompanyID:       companyID,
		CompanyName:     "Acme",
		Title:           "Backend Engineer",
		NormalizedTitle: "backend engineer",
		Description:     "Go and Kafka",
		ContentHash:     "learning-hash-v1",
	})
	if err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}

	result := aiclient.LearningPlanResult{
		Skills:                  []string{"kafka"},
		CurrentReadiness:        40,
		TargetReadiness:         80,
		Topics:                  []string{"kafka fundamentals"},
		PracticeQuestions:       []aiclient.InterviewQuestion{{Question: "What is Kafka?", ConciseAnswer: "A log.", DeeperExplanation: "Details."}},
		Projects:                []string{"Build a Kafka consumer"},
		ArchitectureQuestions:   []string{"How would you scale Kafka consumers?"},
		EstimatedEffortCategory: "QUICK_PREP",
	}

	saved, err := repo.SaveLearningPlan(ctx, user.ID, upserted.Job.ID, result)
	if err != nil {
		t.Fatalf("SaveLearningPlan: %v", err)
	}
	if saved.EstimatedEffortCategory != "QUICK_PREP" {
		t.Fatalf("expected effort category to persist, got %q", saved.EstimatedEffortCategory)
	}
	if len(saved.PracticeQuestions) != 1 || saved.PracticeQuestions[0].Question != "What is Kafka?" {
		t.Fatalf("expected practice questions to round-trip, got %+v", saved.PracticeQuestions)
	}

	// Re-saving for the same (user, job) pair must update, not duplicate.
	result.EstimatedEffortCategory = "STANDARD_PREP"
	updated, err := repo.SaveLearningPlan(ctx, user.ID, upserted.Job.ID, result)
	if err != nil {
		t.Fatalf("SaveLearningPlan (update): %v", err)
	}
	if updated.EstimatedEffortCategory != "STANDARD_PREP" {
		t.Fatalf("expected updated effort category to persist, got %q", updated.EstimatedEffortCategory)
	}
}
