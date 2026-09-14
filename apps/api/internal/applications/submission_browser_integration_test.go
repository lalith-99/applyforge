package applications_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/applications"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
)

func TestBrowserCompanionClaimIsUserScopedAndFenced(t *testing.T) {
	q := testdb.OpenTx(t)
	ctx := context.Background()
	svc, _, userID, _, packageID := fixtureApprovedPackage(t, ctx, q)

	intent, err := svc.CreateSubmissionIntent(ctx, userID, packageID)
	if err != nil {
		t.Fatalf("create submission intent: %v", err)
	}

	if _, err := svc.ClaimSubmissionIntentForUser(ctx, uuid.New(), intent.ID, "browser-wrong-user"); !errors.Is(err, applications.ErrStaleSubmissionLease) {
		t.Fatalf("expected cross-user claim to fail as stale/not authorized, got %v", err)
	}

	claimed, err := svc.ClaimSubmissionIntentForUser(ctx, userID, intent.ID, "browser-1")
	if err != nil {
		t.Fatalf("claim own submission intent: %v", err)
	}
	if claimed.Status != applications.SubmissionClaimed || claimed.LeaseGeneration != 1 {
		t.Fatalf("unexpected claimed intent: %+v", claimed)
	}

	if _, err := svc.BeginSubmissionForUser(ctx, userID, intent.ID, "browser-old", claimed.LeaseGeneration); !errors.Is(err, applications.ErrStaleSubmissionLease) {
		t.Fatalf("expected wrong worker to be fenced, got %v", err)
	}

	if _, err := svc.BeginSubmissionForUser(ctx, uuid.New(), intent.ID, "browser-1", claimed.LeaseGeneration); !errors.Is(err, applications.ErrSubmissionIntentNotFound) {
		t.Fatalf("expected cross-user begin to be rejected before fencing mutation, got %v", err)
	}
}

// Keep db imported here so this test file also fails fast if the generated
// browser companion query disappears during a future sqlc regeneration.
var _ = db.ClaimSubmissionIntentForUserParams{}
