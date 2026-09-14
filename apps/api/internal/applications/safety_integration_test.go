package applications_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/applications"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobs"
	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
	"github.com/lalithlochan/applyforge/apps/api/internal/users"
)

func TestService_SaveRejectsUnknownResumeVersion(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	jobsRepo := jobs.NewRepositoryFromQueries(q)
	repo := applications.NewRepositoryFromQueries(q)
	svc := applications.NewService(repo)
	ctx := context.Background()

	userID, jobID := fixtureUserAndJob(t, ctx, userRepo, jobsRepo)
	unknownVersion := uuid.New()

	_, err := svc.Save(ctx, userID, jobID, &unknownVersion, nil)
	if !errors.Is(err, applications.ErrResumeVersionMismatch) {
		t.Fatalf("expected ErrResumeVersionMismatch, got %v", err)
	}
}

func TestService_SaveStillAllowsNoResumeVersion(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	jobsRepo := jobs.NewRepositoryFromQueries(q)
	repo := applications.NewRepositoryFromQueries(q)
	svc := applications.NewService(repo)
	ctx := context.Background()

	userID, jobID := fixtureUserAndJob(t, ctx, userRepo, jobsRepo)

	if _, err := svc.Save(ctx, userID, jobID, nil, nil); err != nil {
		t.Fatalf("Save without resume version: %v", err)
	}
}
