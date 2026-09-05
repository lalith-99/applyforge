package resumeversion
package resumeversion_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/resume"
	"github.com/lalithlochan/applyforge/apps/api/internal/resumeversion"
	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
	"github.com/lalithlochan/applyforge/apps/api/internal/users"
)

func TestRepository_CreateSetDocumentsAndList(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	resumeRepo := resume.NewRepositoryFromQueries(q)
	repo := resumeversion.NewRepositoryFromQueries(q)
	ctx := context.Background()

	email := fmt.Sprintf("resumeversion-%s@example.com", uuid.NewString())
	user, err := userRepo.CreateWithPassword(ctx, email, "hash")
	if err != nil {
		t.Fatalf("create fixture user: %v", err)
	}

	baseResume, err := resumeRepo.Create(ctx, user.ID, "resume.pdf", "application/pdf", 1024, "resumes/tmp/key")
	if err != nil {
		t.Fatalf("create fixture resume: %v", err)
	}

	next, err := repo.NextVersionNumber(ctx, baseResume.ID)
	if err != nil {
		t.Fatalf("NextVersionNumber: %v", err)
	}
	if next != 1 {
		t.Fatalf("expected first version number to be 1, got %d", next)
	}

	name := "Ada Lovelace"
	content := aiclient.ResumeProfile{
		Contact: aiclient.ContactInfo{Name: &name},
		Skills:  []string{"Go", "Kafka"},
	}
	matchScore := int32(85)
	alignmentScore := int32(70)
	mode := "GROWTH"

	created, err := repo.Create(ctx, user.ID, baseResume.ID, nil, nil, next, content, &matchScore, &alignmentScore, &mode)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.VersionNumber != 1 {
		t.Fatalf("expected version_number 1, got %d", created.VersionNumber)
	}
	if created.Content.Skills[0] != "Go" {
		t.Fatalf("expected content to round-trip, got %+v", created.Content)
	}
	if created.PDFStorageKey != nil || created.DocxStorageKey != nil {
		t.Fatalf("expected no storage keys before documents are generated")
	}

	updated, err := repo.SetDocuments(ctx, created.ID, "resume-versions/x/resume.pdf", "resume-versions/x/resume.docx")
	if err != nil {
		t.Fatalf("SetDocuments: %v", err)
	}
	if updated.PDFStorageKey == nil || *updated.PDFStorageKey != "resume-versions/x/resume.pdf" {
		t.Fatalf("expected pdf storage key to persist, got %v", updated.PDFStorageKey)
	}

	fetched, err := repo.GetForUser(ctx, created.ID, user.ID)
	if err != nil {
		t.Fatalf("GetForUser: %v", err)
	}
	if fetched.DocxStorageKey == nil || *fetched.DocxStorageKey != "resume-versions/x/resume.docx" {
		t.Fatalf("expected docx storage key to persist, got %v", fetched.DocxStorageKey)
	}

	nextAfterFirst, err := repo.NextVersionNumber(ctx, baseResume.ID)
	if err != nil {
		t.Fatalf("NextVersionNumber (second): %v", err)
	}
	if nextAfterFirst != 2 {
		t.Fatalf("expected next version number to be 2 after one version exists, got %d", nextAfterFirst)
	}

	list, err := repo.ListForResume(ctx, baseResume.ID)
	if err != nil {
		t.Fatalf("ListForResume: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 version listed, got %d", len(list))
	}
}

func TestRepository_GetForUser_NotFoundForOtherUser(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	resumeRepo := resume.NewRepositoryFromQueries(q)
	repo := resumeversion.NewRepositoryFromQueries(q)
	ctx := context.Background()

	owner, err := userRepo.CreateWithPassword(ctx, fmt.Sprintf("owner-%s@example.com", uuid.NewString()), "hash")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	other, err := userRepo.CreateWithPassword(ctx, fmt.Sprintf("other-%s@example.com", uuid.NewString()), "hash")
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}

	baseResume, err := resumeRepo.Create(ctx, owner.ID, "resume.pdf", "application/pdf", 1024, "resumes/tmp/key2")
	if err != nil {
		t.Fatalf("create fixture resume: %v", err)
	}

	created, err := repo.Create(ctx, owner.ID, baseResume.ID, nil, nil, 1, aiclient.ResumeProfile{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := repo.GetForUser(ctx, created.ID, other.ID); err != resumeversion.ErrNotFound {
		t.Fatalf("expected ErrNotFound for a different user, got %v", err)
	}
}
