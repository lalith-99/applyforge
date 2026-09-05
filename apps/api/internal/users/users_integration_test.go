package users

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
)

func TestRepository_CreateWithPasswordAndLookup(t *testing.T) {
	q := testdb.OpenTx(t)
	repo := newRepository(q)
	ctx := context.Background()

	email := fmt.Sprintf("password-%s@example.com", uuid.NewString())
	created, err := repo.CreateWithPassword(ctx, email, "some-bcrypt-hash")
	if err != nil {
		t.Fatalf("CreateWithPassword: %v", err)
	}
	if created.Email != email {
		t.Fatalf("expected email %q, got %q", email, created.Email)
	}
	if created.PasswordHash == nil || *created.PasswordHash != "some-bcrypt-hash" {
		t.Fatalf("expected password hash to round-trip")
	}

	byEmail, err := repo.GetByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if byEmail.ID != created.ID {
		t.Fatalf("expected same user id, got %s vs %s", byEmail.ID, created.ID)
	}

	byID, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if byID.Email != email {
		t.Fatalf("expected email %q, got %q", email, byID.Email)
	}
}

func TestRepository_GetByEmail_NotFound(t *testing.T) {
	q := testdb.OpenTx(t)
	repo := newRepository(q)

	_, err := repo.GetByEmail(context.Background(), "does-not-exist@example.com")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRepository_CreateWithGoogleAndLinkGoogleAccount(t *testing.T) {
	q := testdb.OpenTx(t)
	repo := newRepository(q)
	ctx := context.Background()

	email := fmt.Sprintf("google-%s@example.com", uuid.NewString())
	googleID := uuid.NewString()

	created, err := repo.CreateWithGoogle(ctx, email, googleID)
	if err != nil {
		t.Fatalf("CreateWithGoogle: %v", err)
	}
	if created.GoogleID == nil || *created.GoogleID != googleID {
		t.Fatalf("expected google id to round-trip")
	}
	if created.EmailVerifiedAt == nil {
		t.Fatalf("expected google sign-up to mark email verified")
	}

	byGoogle, err := repo.GetByGoogleID(ctx, googleID)
	if err != nil {
		t.Fatalf("GetByGoogleID: %v", err)
	}
	if byGoogle.ID != created.ID {
		t.Fatalf("expected same user id")
	}

	passwordEmail := fmt.Sprintf("link-%s@example.com", uuid.NewString())
	passwordUser, err := repo.CreateWithPassword(ctx, passwordEmail, "hash")
	if err != nil {
		t.Fatalf("CreateWithPassword: %v", err)
	}

	linkedGoogleID := uuid.NewString()
	linked, err := repo.LinkGoogleAccount(ctx, passwordUser.ID, linkedGoogleID)
	if err != nil {
		t.Fatalf("LinkGoogleAccount: %v", err)
	}
	if linked.GoogleID == nil || *linked.GoogleID != linkedGoogleID {
		t.Fatalf("expected linked google id to round-trip")
	}
}
