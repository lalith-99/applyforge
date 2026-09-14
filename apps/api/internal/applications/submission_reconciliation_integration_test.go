package applications_test

import (
	"context"
	"testing"

	"github.com/lalithlochan/applyforge/apps/api/internal/applications"
	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
)

func TestReconcileUncertainAsAppliedMarksApplicationApplied(t *testing.T) {
	q := testdb.OpenTx(t)
	ctx := context.Background()
	svc, repo, userID, applicationID, packageID := fixtureApprovedPackage(t, ctx, q)

	intent, err := svc.CreateSubmissionIntent(ctx, userID, packageID)
	if err != nil {
		t.Fatalf("create intent: %v", err)
	}
	claimed, err := svc.ClaimSubmissionIntentForUser(ctx, userID, intent.ID, "reconcile-applied")
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if _, err := svc.BeginSubmissionForUser(ctx, userID, intent.ID, "reconcile-applied", claimed.LeaseGeneration); err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := svc.MarkSubmissionUncertainForUser(ctx, userID, intent.ID, "reconcile-applied", claimed.LeaseGeneration, "test uncertainty"); err != nil {
		t.Fatalf("mark uncertain: %v", err)
	}

	resolved, err := svc.ReconcileSubmission(ctx, userID, intent.ID, applications.ReconcileApplied, "verified in employer portal")
	if err != nil {
		t.Fatalf("reconcile applied: %v", err)
	}
	if resolved.Status != applications.SubmissionConfirmed {
		t.Fatalf("expected confirmed reconciliation, got %s", resolved.Status)
	}
	app, err := repo.GetForUser(ctx, applicationID, userID)
	if err != nil {
		t.Fatalf("reload application: %v", err)
	}
	if app.Status != applications.StatusApplied || app.AppliedAt == nil {
		t.Fatalf("verified applied reconciliation must update application: %+v", app)
	}
}

func TestReconcileNotSubmittedAllowsSafeRetry(t *testing.T) {
	q := testdb.OpenTx(t)
	ctx := context.Background()
	svc, _, userID, _, packageID := fixtureApprovedPackage(t, ctx, q)

	intent, err := svc.CreateSubmissionIntent(ctx, userID, packageID)
	if err != nil {
		t.Fatalf("create intent: %v", err)
	}
	claimed, err := svc.ClaimSubmissionIntentForUser(ctx, userID, intent.ID, "reconcile-retry")
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if _, err := svc.BeginSubmissionForUser(ctx, userID, intent.ID, "reconcile-retry", claimed.LeaseGeneration); err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := svc.MarkSubmissionUncertainForUser(ctx, userID, intent.ID, "reconcile-retry", claimed.LeaseGeneration, "test uncertainty"); err != nil {
		t.Fatalf("mark uncertain: %v", err)
	}

	cancelled, err := svc.ReconcileSubmission(ctx, userID, intent.ID, applications.ReconcileNotSubmitted, "verified no application in portal")
	if err != nil {
		t.Fatalf("reconcile not submitted: %v", err)
	}
	if cancelled.Status != applications.SubmissionCancelled {
		t.Fatalf("expected cancelled reconciliation, got %s", cancelled.Status)
	}

	reactivated, err := svc.CreateSubmissionIntent(ctx, userID, packageID)
	if err != nil {
		t.Fatalf("reactivate after explicit no-submit verification: %v", err)
	}
	if reactivated.ID != intent.ID || reactivated.Status != applications.SubmissionPending {
		t.Fatalf("expected same intent safely returned to pending, got %+v", reactivated)
	}
	if reactivated.LeaseGeneration != claimed.LeaseGeneration {
		t.Fatalf("fencing generation must not roll back during reconciliation: %+v", reactivated)
	}
}
