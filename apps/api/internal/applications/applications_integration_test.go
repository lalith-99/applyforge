package applications_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/applications"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobs"
	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
	"github.com/lalithlochan/applyforge/apps/api/internal/users"
)

func fixtureUserAndJob(t *testing.T, ctx context.Context, userRepo *users.Repository, jobsRepo *jobs.Repository) (uuid.UUID, uuid.UUID) {
	t.Helper()
	user, err := userRepo.CreateWithPassword(ctx, fmt.Sprintf("applications-%s@example.com", uuid.NewString()), "hash")
	if err != nil {
		t.Fatalf("create fixture user: %v", err)
	}

	externalID := uuid.NewString()
	companyID, err := jobsRepo.UpsertCompany(ctx, "Acme", fmt.Sprintf("acme-applications-%s", externalID))
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
		ContentHash:     "applications-hash-" + externalID,
	})
	if err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}
	return user.ID, upserted.Job.ID
}

func TestRepository_CreateIsUpsertNotDuplicate(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	jobsRepo := jobs.NewRepositoryFromQueries(q)
	repo := applications.NewRepositoryFromQueries(q)
	ctx := context.Background()

	userID, jobID := fixtureUserAndJob(t, ctx, userRepo, jobsRepo)

	first, err := repo.Create(ctx, userID, jobID, nil, applications.StatusSaved, nil)
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	second, err := repo.Create(ctx, userID, jobID, nil, applications.StatusSaved, nil)
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected repeated Create for the same user+job to return the same row")
	}

	list, err := repo.ListForUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListForUser: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected exactly 1 application, got %d", len(list))
	}
}

func TestService_ChangeStatus_LogsEventAndRejectsInvalidStatus(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	jobsRepo := jobs.NewRepositoryFromQueries(q)
	repo := applications.NewRepositoryFromQueries(q)
	svc := applications.NewService(repo)
	ctx := context.Background()

	userID, jobID := fixtureUserAndJob(t, ctx, userRepo, jobsRepo)

	app, err := repo.Create(ctx, userID, jobID, nil, applications.StatusSaved, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := svc.ChangeStatus(ctx, userID, app.ID, "NOT_A_REAL_STATUS", nil); err != applications.ErrInvalidStatus {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}

	updated, err := svc.ChangeStatus(ctx, userID, app.ID, applications.StatusApplied, nil)
	if err != nil {
		t.Fatalf("ChangeStatus: %v", err)
	}
	if updated.Status != applications.StatusApplied {
		t.Fatalf("expected status APPLIED, got %q", updated.Status)
	}
	if updated.AppliedAt == nil {
		t.Fatalf("expected applied_at to be set on first transition to APPLIED")
	}

	events, err := repo.ListEvents(ctx, app.ID)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event logged, got %d", len(events))
	}
	if events[0].FromStatus == nil || *events[0].FromStatus != applications.StatusSaved {
		t.Fatalf("expected from_status SAVED, got %v", events[0].FromStatus)
	}
	if events[0].ToStatus == nil || *events[0].ToStatus != applications.StatusApplied {
		t.Fatalf("expected to_status APPLIED, got %v", events[0].ToStatus)
	}
}

func TestService_ChangeStatus_NoOpTransitionDoesNotLogEvent(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	jobsRepo := jobs.NewRepositoryFromQueries(q)
	repo := applications.NewRepositoryFromQueries(q)
	svc := applications.NewService(repo)
	ctx := context.Background()

	userID, jobID := fixtureUserAndJob(t, ctx, userRepo, jobsRepo)
	app, err := repo.Create(ctx, userID, jobID, nil, applications.StatusSaved, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := svc.ChangeStatus(ctx, userID, app.ID, applications.StatusSaved, nil); err != nil {
		t.Fatalf("ChangeStatus (no-op): %v", err)
	}

	events, err := repo.ListEvents(ctx, app.ID)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no event logged for a no-op status transition, got %d", len(events))
	}
}

func TestRepository_ApplicationAnswers_UpsertAndGet(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	repo := applications.NewRepositoryFromQueries(q)
	ctx := context.Background()

	user, err := userRepo.CreateWithPassword(ctx, fmt.Sprintf("answers-%s@example.com", uuid.NewString()), "hash")
	if err != nil {
		t.Fatalf("create fixture user: %v", err)
	}

	if _, err := repo.GetAnswers(ctx, user.ID); err != applications.ErrNotFound {
		t.Fatalf("expected ErrNotFound before first upsert, got %v", err)
	}

	fullName := "Ada Lovelace"
	created, err := repo.UpsertAnswers(ctx, user.ID, applications.UpsertAnswersInput{FullName: &fullName})
	if err != nil {
		t.Fatalf("UpsertAnswers: %v", err)
	}
	if created.FullName == nil || *created.FullName != fullName {
		t.Fatalf("expected full_name to persist, got %v", created.FullName)
	}

	updatedName := "Ada King"
	updated, err := repo.UpsertAnswers(ctx, user.ID, applications.UpsertAnswersInput{FullName: &updatedName})
	if err != nil {
		t.Fatalf("UpsertAnswers (update): %v", err)
	}
	if updated.FullName == nil || *updated.FullName != updatedName {
		t.Fatalf("expected full_name to update, got %v", updated.FullName)
	}

	fetched, err := repo.GetAnswers(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetAnswers: %v", err)
	}
	if fetched.FullName == nil || *fetched.FullName != updatedName {
		t.Fatalf("expected updated full_name to round-trip, got %v", fetched.FullName)
	}
}
