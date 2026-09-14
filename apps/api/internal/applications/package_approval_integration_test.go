package applications_test

import (
	"context"
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

func TestApplicationPackage_BuildApproveAndRevoke(t *testing.T) {
	q := testdb.OpenTx(t)
	ctx := context.Background()
	userRepo := users.NewRepositoryFromQueries(q)
	jobsRepo := jobs.NewRepositoryFromQueries(q)
	resumeRepo := resume.NewRepositoryFromQueries(q)
	appRepo := applications.NewRepositoryFromQueries(q)
	svc := applications.NewService(appRepo)

	user, err := userRepo.CreateWithPassword(ctx, fmt.Sprintf("package-%s@example.com", uuid.NewString()), "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	externalID := uuid.NewString()
	companyID, err := jobsRepo.UpsertCompany(ctx, "Package Test Co", "package-test-"+externalID)
	if err != nil {
		t.Fatalf("create company: %v", err)
	}
	applyURL := "https://jobs.example.com/apply/" + externalID
	upserted, err := jobsRepo.UpsertJob(ctx, jobs.Job{
		Source:          "GREENHOUSE",
		ExternalID:      externalID,
		CompanyID:       companyID,
		CompanyName:     "Package Test Co",
		Title:           "Backend Engineer",
		NormalizedTitle: "backend engineer",
		Description:     "Build APIs",
		ContentHash:     "package-test-" + externalID,
		ApplyURL:         &applyURL,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	master, err := resumeRepo.Create(ctx, user.ID, "resume.pdf", "application/pdf", 100, "test/resume.pdf")
	if err != nil {
		t.Fatalf("create resume: %v", err)
	}
	versionContent := []byte(`{"summary":"Backend engineer","skills":["Go","Kafka"]}`)
	version, err := q.CreateResumeVersion(ctx, db.CreateResumeVersionParams{
		UserID: database.UUIDToPG(user.ID), BaseResumeID: database.UUIDToPG(master.ID), JobID: database.UUIDToPG(upserted.Job.ID),
		TailoringRunID: pgtype.UUID{}, VersionNumber: 1, ContentJson: versionContent,
		MatchScore: pgtype.Int4{}, AlignmentScore: pgtype.Int4{}, TailoringMode: pgtype.Text{},
	})
	if err != nil {
		t.Fatalf("create resume version: %v", err)
	}
	versionID := database.PGToUUID(version.ID)

	fullName := "Ada Example"
	if _, err := appRepo.UpsertAnswers(ctx, user.ID, applications.UpsertAnswersInput{
		FullName: &fullName,
		CommonAnswers: []byte(`{"authorized":true,"sponsorship":"required"}`),
	}); err != nil {
		t.Fatalf("save answers: %v", err)
	}

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
	if pkg.DestinationURL != applyURL {
		t.Fatalf("expected destination %q, got %q", applyURL, pkg.DestinationURL)
	}
	if pkg.PackageHash == "" || len(pkg.PackageHash) != 64 {
		t.Fatalf("expected SHA-256 package hash, got %q", pkg.PackageHash)
	}
	if pkg.ResumeContentHash == "" || pkg.AnswersHash == "" {
		t.Fatal("expected resume and answer hashes")
	}

	rebuilt, err := svc.BuildApplicationPackage(ctx, user.ID, app.ID)
	if err != nil {
		t.Fatalf("rebuild package: %v", err)
	}
	if rebuilt.ID != pkg.ID || rebuilt.PackageHash != pkg.PackageHash {
		t.Fatalf("expected idempotent package build, first=%s second=%s", pkg.ID, rebuilt.ID)
	}

	if _, err := svc.ApproveApplicationPackage(ctx, user.ID, pkg.ID, "wrong", "wrong"); err != applications.ErrInvalidApproval {
		t.Fatalf("expected invalid confirmation rejection, got %v", err)
	}
	approval, err := svc.ApproveApplicationPackage(ctx, user.ID, pkg.ID, applications.ApprovalConfirmationVersion, applications.ApprovalConfirmationText)
	if err != nil {
		t.Fatalf("approve package: %v", err)
	}
	if approval.PackageHash != pkg.PackageHash || approval.ActionScope != applications.ApprovalActionScope {
		t.Fatalf("approval is not scoped to package: %+v", approval)
	}
	if !approval.ExpiresAt.After(approval.ApprovedAt) {
		t.Fatal("approval should expire after approval time")
	}

	revoked, err := svc.RevokeApplicationPackageApproval(ctx, user.ID, pkg.ID)
	if err != nil {
		t.Fatalf("revoke approval: %v", err)
	}
	if revoked.RevokedAt == nil {
		t.Fatal("expected revoked_at to be set")
	}
}
