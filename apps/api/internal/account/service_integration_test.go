package account_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/account"
	"github.com/lalithlochan/applyforge/apps/api/internal/resume"
	"github.com/lalithlochan/applyforge/apps/api/internal/resumeversion"
	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
	"github.com/lalithlochan/applyforge/apps/api/internal/users"
)

// fakeStorage records deleted keys instead of talking to a real object
// store, so these tests can run without MinIO.
type fakeStorage struct {
	deleted []string
}

func (f *fakeStorage) Delete(_ context.Context, key string) error {
	f.deleted = append(f.deleted, key)
	return nil
}

func TestService_DeleteResume_RemovesRowButLeavesOtherResumes(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	resumeRepo := resume.NewRepositoryFromQueries(q)
	resumeVersionRepo := resumeversion.NewRepositoryFromQueries(q)
	ctx := context.Background()

	user, err := userRepo.CreateWithPassword(ctx, fmt.Sprintf("account-%s@example.com", uuid.NewString()), "hash")
	if err != nil {
		t.Fatalf("create fixture user: %v", err)
	}

	toDelete, err := resumeRepo.Create(ctx, user.ID, "delete-me.pdf", "application/pdf", 10, "resumes/tmp/delete-me")
	if err != nil {
		t.Fatalf("create resume to delete: %v", err)
	}
	toKeep, err := resumeRepo.Create(ctx, user.ID, "keep-me.pdf", "application/pdf", 10, "resumes/tmp/keep-me")
	if err != nil {
		t.Fatalf("create resume to keep: %v", err)
	}

	svc := account.NewService(userRepo, resumeRepo, resumeVersionRepo, &fakeStorage{})

	if err := svc.DeleteResume(ctx, user.ID, toDelete.ID); err != nil {
		t.Fatalf("DeleteResume: %v", err)
	}

	if _, err := resumeRepo.Get(ctx, toDelete.ID, user.ID); err != resume.ErrNotFound {
		t.Fatalf("expected deleted resume to be gone, got %v", err)
	}
	if _, err := resumeRepo.Get(ctx, toKeep.ID, user.ID); err != nil {
		t.Fatalf("expected the other resume to remain, got %v", err)
	}
}

func TestService_DeleteResume_WrongUserReturnsNotFound(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	resumeRepo := resume.NewRepositoryFromQueries(q)
	resumeVersionRepo := resumeversion.NewRepositoryFromQueries(q)
	ctx := context.Background()

	owner, err := userRepo.CreateWithPassword(ctx, fmt.Sprintf("owner-%s@example.com", uuid.NewString()), "hash")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	other, err := userRepo.CreateWithPassword(ctx, fmt.Sprintf("other-%s@example.com", uuid.NewString()), "hash")
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}
	res, err := resumeRepo.Create(ctx, owner.ID, "resume.pdf", "application/pdf", 10, "resumes/tmp/owned")
	if err != nil {
		t.Fatalf("create fixture resume: %v", err)
	}

	svc := account.NewService(userRepo, resumeRepo, resumeVersionRepo, &fakeStorage{})

	if err := svc.DeleteResume(ctx, other.ID, res.ID); err != resume.ErrNotFound {
		t.Fatalf("expected ErrNotFound when deleting another user's resume, got %v", err)
	}
}

func TestService_DeleteAccount_CascadesResumesAndUser(t *testing.T) {
	q := testdb.OpenTx(t)
	userRepo := users.NewRepositoryFromQueries(q)
	resumeRepo := resume.NewRepositoryFromQueries(q)
	resumeVersionRepo := resumeversion.NewRepositoryFromQueries(q)
	ctx := context.Background()

	user, err := userRepo.CreateWithPassword(ctx, fmt.Sprintf("delete-account-%s@example.com", uuid.NewString()), "hash")
	if err != nil {
		t.Fatalf("create fixture user: %v", err)
	}
	res, err := resumeRepo.Create(ctx, user.ID, "resume.pdf", "application/pdf", 10, "resumes/tmp/for-deletion")
	if err != nil {
		t.Fatalf("create fixture resume: %v", err)
	}

	storageFake := &fakeStorage{}
	svc := account.NewService(userRepo, resumeRepo, resumeVersionRepo, storageFake)

	if err := svc.DeleteAccount(ctx, user.ID); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}

	if _, err := userRepo.GetByID(ctx, user.ID); err != users.ErrNotFound {
		t.Fatalf("expected user to be deleted, got %v", err)
	}
	// The resume row should be gone too (ON DELETE CASCADE), not just
	// unreachable via the deleted user's ownership check.
	if _, err := resumeRepo.Get(ctx, res.ID, user.ID); err != resume.ErrNotFound {
		t.Fatalf("expected resume to cascade-delete with the user, got %v", err)
	}
	if len(storageFake.deleted) != 1 || storageFake.deleted[0] != "resumes/tmp/for-deletion" {
		t.Fatalf("expected the resume's storage key to be deleted, got %v", storageFake.deleted)
	}
}
