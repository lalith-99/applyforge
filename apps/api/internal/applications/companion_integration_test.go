package applications_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/lalithlochan/applyforge/apps/api/internal/applications"
	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
)

func TestCompanionHandoffScopesTokenToOneIntentAndConfirms(t *testing.T) {
	q := testdb.OpenTx(t)
	ctx := context.Background()
	svc, repo, userID, applicationID, packageID := fixtureApprovedPackage(t, ctx, q)
	companion := applications.NewCompanionService(svc, repo)

	intent, err := svc.CreateSubmissionIntent(ctx, userID, packageID)
	if err != nil {
		t.Fatalf("create submission intent: %v", err)
	}
	firstHandoff, err := companion.CreateHandoff(ctx, userID, intent.ID)
	if err != nil {
		t.Fatalf("create first companion handoff: %v", err)
	}
	if firstHandoff.Token == "" || firstHandoff.IntentID != intent.ID || firstHandoff.PackageID != packageID {
		t.Fatalf("unexpected first handoff: %+v", firstHandoff)
	}

	handoff, err := companion.CreateHandoff(ctx, userID, intent.ID)
	if err != nil {
		t.Fatalf("rotate companion handoff: %v", err)
	}
	if handoff.Token == firstHandoff.Token {
		t.Fatal("rotated companion handoff must use a fresh capability")
	}
	if _, err := companion.Bundle(ctx, intent.ID, firstHandoff.Token); !errors.Is(err, applications.ErrCompanionUnauthorized) {
		t.Fatalf("rotated-out companion token should be rejected, got %v", err)
	}

	if _, err := companion.Bundle(ctx, intent.ID, "wrong-token"); !errors.Is(err, applications.ErrCompanionUnauthorized) {
		t.Fatalf("expected wrong companion token to be rejected, got %v", err)
	}
	bundle, err := companion.Bundle(ctx, intent.ID, handoff.Token)
	if err != nil {
		t.Fatalf("load companion bundle: %v", err)
	}
	if bundle.Intent.ID != intent.ID || bundle.Package.ID != packageID || len(bundle.ResumeContent) == 0 {
		t.Fatalf("unexpected companion bundle: %+v", bundle)
	}

	claimed, err := companion.Claim(ctx, intent.ID, handoff.Token, "companion-test")
	if err != nil {
		t.Fatalf("claim via companion: %v", err)
	}
	if claimed.LeaseGeneration != 1 || claimed.Status != applications.SubmissionClaimed {
		t.Fatalf("unexpected claim: %+v", claimed)
	}
	if _, err := companion.Begin(ctx, intent.ID, handoff.Token, "companion-test", claimed.LeaseGeneration); err != nil {
		t.Fatalf("begin companion submission: %v", err)
	}
	confirmed, err := companion.Confirm(ctx, intent.ID, handoff.Token, "companion-test", claimed.LeaseGeneration, json.RawMessage(`{"signal":"integration-test"}`))
	if err != nil {
		t.Fatalf("confirm companion submission: %v", err)
	}
	if confirmed.Status != applications.SubmissionConfirmed {
		t.Fatalf("expected confirmed intent, got %s", confirmed.Status)
	}

	app, err := repo.GetForUser(ctx, applicationID, userID)
	if err != nil {
		t.Fatalf("reload application: %v", err)
	}
	if app.Status != applications.StatusApplied {
		t.Fatalf("companion confirmation must mark application applied, got %s", app.Status)
	}

	if _, err := companion.Bundle(ctx, intent.ID, handoff.Token); !errors.Is(err, applications.ErrCompanionUnauthorized) {
		t.Fatalf("confirmed handoff token should be revoked, got %v", err)
	}
}
