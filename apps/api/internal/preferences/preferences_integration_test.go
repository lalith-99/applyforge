package preferences

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
	"github.com/lalithlochan/applyforge/apps/api/internal/users"
)

func TestRepository_UpsertAndGet_WithImmigrationPreferences(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	repo := newRepository(q)
	ctx := context.Background()

	email := fmt.Sprintf("preferences-%s@example.com", uuid.NewString())
	user, err := userRepo.CreateWithPassword(ctx, email, "hash")
	if err != nil {
		t.Fatalf("create fixture user: %v", err)
	}

	status := "H1B"
	confidence := "MEDIUM"
	in := UpsertInput{
		Remote:                          true,
		PreferredLocations:              []string{"Remote - US"},
		EmploymentTypes:                 []string{"full_time"},
		ImmigrationStatus:               &status,
		RequiresH1BTransfer:             true,
		RequiresNewH1BCapSponsorship:    false,
		GreenCardSupportPreferred:       true,
		PermSupportPreferred:            true,
		ImmigrationSupportMinConfidence: &confidence,
	}

	created, err := repo.Upsert(ctx, user.ID, in)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if !created.RequiresH1BTransfer {
		t.Fatalf("expected requires_h1b_transfer to be true")
	}
	if created.RequiresNewH1BCapSponsorship {
		t.Fatalf("expected requires_new_h1b_cap_sponsorship to be false")
	}
	if created.ImmigrationStatus == nil || *created.ImmigrationStatus != status {
		t.Fatalf("expected immigration_status %q, got %v", status, created.ImmigrationStatus)
	}
	// Requiring H-1B transfer support must remain a distinct signal from
	// requiring brand-new H-1B cap sponsorship (see MASTER_REQUIREMENTS.md).
	if created.RequiresH1BTransfer == created.RequiresNewH1BCapSponsorship {
		t.Fatalf("h1b transfer and new h1b cap sponsorship must be independently tracked")
	}

	fetched, err := repo.Get(ctx, user.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !fetched.GreenCardSupportPreferred || !fetched.PermSupportPreferred {
		t.Fatalf("expected green card / perm preferences to persist")
	}
}

func TestRepository_GetBeforeUpsertReturnsNotFound(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	repo := newRepository(q)
	ctx := context.Background()

	email := fmt.Sprintf("preferences-%s@example.com", uuid.NewString())
	user, err := userRepo.CreateWithPassword(ctx, email, "hash")
	if err != nil {
		t.Fatalf("create fixture user: %v", err)
	}

	_, err = repo.Get(ctx, user.ID)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
