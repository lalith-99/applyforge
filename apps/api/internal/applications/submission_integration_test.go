package applications_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/lalithlochan/applyforge/apps/api/internal/applications"
	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobs"
	"github.com/lalithlochan/applyforge/apps/api/internal/resume"
	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
	"github.com/lalithlochan/applyforge/apps/api/internal/users"
)

func TestSubmissionIntent_FencesStaleWorkersAndConfirmsApplication(t *testing.T) {
	q := testdb.OpenTx(t)
	ctx := context.Background()
	svc, repo, userID, applicationID, packageID := fixtureApprovedPackage(t, ctx, q)

	intent, err := svc.CreateSubmissionIntent(ctx, userID, packageID)
	if err != nil {
		t.Fatalf("create submission intent: %v", err)
	}
	if intent.Status != applications.SubmissionPending {
		t.Fatalf("expected pending intent, got %s", intent.Status)
	}

	again, err := svc.CreateSubmissionIntent(ctx, userID, packageID)
	if err != nil {
		t.Fatalf("recreate submission intent: %v", err)
	}
	if again.ID != intent.ID || again.IdempotencyKey != intent.IdempotencyKey {
		t.Fatal("submission intent creation must be idempotent per package")
	}

	claimed, err := svc.ClaimNextSubmissionIntent(ctx, "executor-1")
	if err != nil {
		t.Fatalf("claim submission: %v", err)
	}
	if claimed.ID != intent.ID || claimed.Status != applications.SubmissionClaimed || claimed.LeaseGeneration != 1 {
		t.Fatalf("unexpected claim: %+v", claimed)
	}

	if _, err := svc.BeginSubmission(ctx, intent.ID, "executor-1", claimed.LeaseGeneration+1); !errors.Is(err, applications.ErrStaleSubmissionLease) {
		t.Fatalf("expected stale generation to be fenced, got %v", err)
	}

	submitting, err := svc.BeginSubmission(ctx, intent.ID, "executor-1", claimed.LeaseGeneration)
	if err != nil {
		t.Fatalf("begin submission: %v", err)
	}
	if submitting.Status != applications.SubmissionSubmitting {
		t.Fatalf("expected SUBMITTING, got %s", submitting.Status)
	}

	if _, err := svc.ConfirmSubmission(ctx, intent.ID, "executor-old", claimed.LeaseGeneration, json.RawMessage(`{"provider_id":"wrong"}`)); !errors.Is(err, applications.ErrStaleSubmissionLease) {
		t.Fatalf("expected stale worker to be fenced at confirmation, got %v", err)
	}

	confirmed, err := svc.ConfirmSubmission(ctx, intent.ID, "executor-1", claimed.LeaseGeneration, json.RawMessage(`{"provider_id":"receipt-123"}`))
	if err != nil {
		t.Fatalf("confirm submission: %v", err)
	}
	if confirmed.Status != applications.SubmissionConfirmed || confirmed.CompletedAt == nil {
		t.Fatalf("expected confirmed intent, got %+v", confirmed)
	}

	app, err := repo.GetForUser(ctx, applicationID, userID)
	if err != nil {
		t.Fatalf("reload application: %v", err)
	}
	if app.Status != applications.StatusApplied || app.AppliedAt == nil {
		t.Fatalf("confirmed submission must atomically mark application applied: %+v", app)
	}
}

func TestSubmissionIntent_RequiresLiveApproval(t *testing.T) {
	q := testdb.OpenTx(t)
	ctx := context.Background()
	svc, _, userID, _, packageID := fixtureApprovedPackage(t, ctx, q)

	if _, err := svc.RevokeApplicationPackageApproval(ctx, userID, packageID); err != nil {
		t.Fatalf("revoke approval: %v", err)
	}
	if _, err := svc.CreateSubmissionIntent(ctx, userID, packageID); !errors.Is(err, applications.ErrActiveApprovalRequired) {
		t.Fatalf("expected active approval requirement, got %v", err)
	}
}

func fixtureApprovedPackage(t *testing.T, ctx context.Context, q *db.Queries) (*applications.Service, *applications.Repository, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	userRepo := users.NewRepositoryFromQueries(q)
	jobsRepo := jobs.NewRepositoryFromQueries(q)
	resumeRepo := resume.NewRepositoryFromQueries(q)
	appRepo := applications.NewRepositoryFromQueries(q)
	svc := applications.NewService(appRepo)

	user, err := userRepo.CreateWithPassword(ctx, fmt.Sprintf("submission-%s@example.com", uuid.NewString()), "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	externalID := uuid.NewString()
	companyID, err := jobsRepo.UpsertCompany(ctx, "Submission Test Co", "submission-test-"+externalID)
	if err != nil {
		t.Fatalf("create company: %v", err)
	}
	applyURL := "https://jobs.example.com/apply/" + externalID
	upserted, err := jobsRepo.UpsertJob(ctx, jobs.Job{
		Source:          "GREENHOUSE",
		ExternalID:      externalID,
		CompanyID:       companyID,
		CompanyName:     "Submission Test Co",
		Title:           "Platform Engineer",
		NormalizedTitle: "platform engineer",
		Description:     "Build reliable systems",
		ContentHash:     "submission-test-" + externalID,
		ApplyURL:        &applyURL,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	master, err := resumeRepo.Create(ctx, user.ID, "resume.pdf", "application/pdf", 100, "test/submission-resume.pdf")
	if err != nil {
		t.Fatalf("create resume: %v", err)
	}
	version, err := q.CreateResumeVersion(ctx, db.CreateResumeVersionParams{
		UserID:          database.UUIDToPG(user.ID),
		BaseResumeID:    database.UUIDToPG(master.ID),
		JobID:           database.UUIDToPG(upserted.Job.ID),
		TailoringRunID:  pgtype.UUID{},
		VersionNumber:   1,
		ContentJson:     []byte(`{"summary":"Platform engineer"}`),
		MatchScore:      pgtype.Int4{},
		AlignmentScore:  pgtype.Int4{},
		TailoringMode:   pgtype.Text{},
	})
	if err != nil {
		t.Fatalf("create resume version: %v", err)
	}
	versionID := database.PGToUUID(version.ID)

	app, err := svc.Save(ctx, user.ID, upserted.Job.ID, &versionID, nil)
	if err != nil {
		t.Fatalf("save application: %v", err)
	}
	app, err = svc.ChangeStatus(ctx, user.ID, app.ID, applications.StatusReadyToApply, nil)
	if err != nil {
		t.Fatalf("mark ready: %v", err)
	}
	pkg, err := svc.BuildApplicationPackage(ctx, user.ID, app.ID)
	if err != nil {
		t.Fatalf("build package: %v", err)
	}
	if _, err := svc.ApproveApplicationPackage(ctx, user.ID, pkg.ID, applications.ApprovalConfirmationVersion, applications.ApprovalConfirmationText); err != nil {
		t.Fatalf("approve package: %v", err)
	}
	return svc, appRepo, user.ID, app.ID, pkg.ID
}
