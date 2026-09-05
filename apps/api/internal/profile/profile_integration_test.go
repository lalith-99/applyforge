package profile

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
	"github.com/lalithlochan/applyforge/apps/api/internal/users"
)

func TestRepository_GetBeforeUpsertReturnsNotFound(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	repo := newRepository(q)
	ctx := context.Background()

	email := fmt.Sprintf("profile-%s@example.com", uuid.NewString())
	user, err := userRepo.CreateWithPassword(ctx, email, "hash")
	if err != nil {
		t.Fatalf("create fixture user: %v", err)
	}

	_, err = repo.Get(ctx, user.ID)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRepository_UpsertAndGet(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	repo := newRepository(q)
	ctx := context.Background()

	email := fmt.Sprintf("profile-%s@example.com", uuid.NewString())
	user, err := userRepo.CreateWithPassword(ctx, email, "hash")
	if err != nil {
		t.Fatalf("create fixture user: %v", err)
	}

	firstName := "Ada"
	years := int32(6)
	in := UpsertInput{
		FirstName:               &firstName,
		PrimaryTargetTitles:     []string{"Backend Engineer", "Software Engineer"},
		AlternativeTargetTitles: []string{"Platform Engineer"},
		YearsExperience:         &years,
		PreferredTechnologies:   []string{"Go", "PostgreSQL"},
	}

	created, err := repo.Upsert(ctx, user.ID, in)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if created.FirstName == nil || *created.FirstName != firstName {
		t.Fatalf("expected first name %q, got %v", firstName, created.FirstName)
	}
	if len(created.PrimaryTargetTitles) != 2 {
		t.Fatalf("expected 2 primary target titles, got %d", len(created.PrimaryTargetTitles))
	}
	if created.OnboardingCompletedAt != nil {
		t.Fatalf("expected onboarding not yet complete")
	}

	in.MarkOnboardingComplete = true
	updated, err := repo.Upsert(ctx, user.ID, in)
	if err != nil {
		t.Fatalf("Upsert (complete onboarding): %v", err)
	}
	if updated.OnboardingCompletedAt == nil {
		t.Fatalf("expected onboarding_completed_at to be set")
	}

	fetched, err := repo.Get(ctx, user.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fetched.FirstName == nil || *fetched.FirstName != firstName {
		t.Fatalf("expected persisted first name %q, got %v", firstName, fetched.FirstName)
	}
}
