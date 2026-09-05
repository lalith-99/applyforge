package auth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
	"github.com/lalithlochan/applyforge/apps/api/internal/users"
)

func TestSessionRepository_CreateGetDelete(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	sessions := newSessionRepositoryFromQueries(q)
	ctx := context.Background()

	email := fmt.Sprintf("session-%s@example.com", uuid.NewString())
	user, err := userRepo.CreateWithPassword(ctx, email, "hash")
	if err != nil {
		t.Fatalf("create fixture user: %v", err)
	}

	tokenHash := hashToken("raw-token-value")
	created, err := sessions.create(ctx, user.ID, tokenHash, "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.UserID != user.ID {
		t.Fatalf("expected session to belong to fixture user")
	}
	if created.ExpiresAt.Before(time.Now()) {
		t.Fatalf("expected session to expire in the future")
	}

	fetched, err := sessions.getByTokenHash(ctx, tokenHash)
	if err != nil {
		t.Fatalf("getByTokenHash: %v", err)
	}
	if fetched.ID != created.ID {
		t.Fatalf("expected same session id")
	}

	if err := sessions.deleteByTokenHash(ctx, tokenHash); err != nil {
		t.Fatalf("deleteByTokenHash: %v", err)
	}

	if _, err := sessions.getByTokenHash(ctx, tokenHash); err != ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound after delete, got %v", err)
	}
}
