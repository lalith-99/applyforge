package resume

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
	"github.com/lalithlochan/applyforge/apps/api/internal/users"
)

func TestRepository_UploadParseLifecycle(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	repo := newRepository(q)
	ctx := context.Background()

	email := fmt.Sprintf("resume-%s@example.com", uuid.NewString())
	user, err := userRepo.CreateWithPassword(ctx, email, "hash")
	if err != nil {
		t.Fatalf("create fixture user: %v", err)
	}

	created, err := repo.Create(ctx, user.ID, "resume.pdf", "application/pdf", 1024, "resumes/tmp/key")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Status != StatusUploaded {
		t.Fatalf("expected initial status %q, got %q", StatusUploaded, created.Status)
	}

	if err := repo.MarkParsing(ctx, created.ID); err != nil {
		t.Fatalf("MarkParsing: %v", err)
	}

	profile := json.RawMessage(`{"skills":["Go"]}`)
	if err := repo.MarkParsed(ctx, created.ID, "raw text", profile); err != nil {
		t.Fatalf("MarkParsed: %v", err)
	}

	fetched, err := repo.Get(ctx, created.ID, user.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fetched.Status != StatusParsed {
		t.Fatalf("expected status %q, got %q", StatusParsed, fetched.Status)
	}
	if fetched.RawText == nil || *fetched.RawText != "raw text" {
		t.Fatalf("expected raw text to persist")
	}

	experiences := []Experience{{Company: strPtr("Acme"), Title: strPtr("Engineer"), Bullets: []string{"Did things"}}}
	if err := repo.ReplaceExperiences(ctx, created.ID, experiences); err != nil {
		t.Fatalf("ReplaceExperiences: %v", err)
	}

	listed, err := repo.ListExperiences(ctx, created.ID)
	if err != nil {
		t.Fatalf("ListExperiences: %v", err)
	}
	if len(listed) != 1 || listed[0].Company == nil || *listed[0].Company != "Acme" {
		t.Fatalf("expected one experience for Acme, got %+v", listed)
	}

	all, err := repo.ListForUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListForUser: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 resume for user, got %d", len(all))
	}
}

func TestRepository_MarkFailed(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	repo := newRepository(q)
	ctx := context.Background()

	email := fmt.Sprintf("resume-fail-%s@example.com", uuid.NewString())
	user, err := userRepo.CreateWithPassword(ctx, email, "hash")
	if err != nil {
		t.Fatalf("create fixture user: %v", err)
	}

	created, err := repo.Create(ctx, user.ID, "bad.pdf", "application/pdf", 10, "resumes/tmp/bad")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.MarkFailed(ctx, created.ID, "extraction failed"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	fetched, err := repo.Get(ctx, created.ID, user.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fetched.Status != StatusFailed {
		t.Fatalf("expected status %q, got %q", StatusFailed, fetched.Status)
	}
	if fetched.ParseError == nil || *fetched.ParseError != "extraction failed" {
		t.Fatalf("expected parse error to persist")
	}
}

func strPtr(s string) *string { return &s }
