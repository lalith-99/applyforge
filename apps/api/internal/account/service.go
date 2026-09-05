// Package account handles account-level operations that span multiple
// domains: resume deletion and full account deletion (see
// MASTER_REQUIREMENTS.md §46, "production hardening" — Phase 12). All
// foreign keys referencing users/resumes use ON DELETE CASCADE, so the
// database cleans up every dependent row automatically; this package's job
// is only to delete the object-storage files a DB cascade can't reach.
package account

import (
	"context"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/resume"
	"github.com/lalithlochan/applyforge/apps/api/internal/resumeversion"
	"github.com/lalithlochan/applyforge/apps/api/internal/users"
)

// StorageDeleter is the minimal storage capability this package needs — a
// subset of *storage.Client, so tests can inject a fake instead of a real
// MinIO connection.
type StorageDeleter interface {
	Delete(ctx context.Context, key string) error
}

// Service coordinates deleting a resume's (or a whole account's) object
// storage files before removing the owning database rows.
type Service struct {
	usersRepo         *users.Repository
	resumeRepo        *resume.Repository
	resumeVersionRepo *resumeversion.Repository
	storageClient     StorageDeleter
}

// NewService builds a Service.
func NewService(usersRepo *users.Repository, resumeRepo *resume.Repository, resumeVersionRepo *resumeversion.Repository, storageClient StorageDeleter) *Service {
	return &Service{
		usersRepo:         usersRepo,
		resumeRepo:        resumeRepo,
		resumeVersionRepo: resumeVersionRepo,
		storageClient:     storageClient,
	}
}

// DeleteResume removes a resume's generated documents and uploaded file from
// object storage, then deletes the resume row (cascading its experiences and
// versions in the database).
func (s *Service) DeleteResume(ctx context.Context, userID, resumeID uuid.UUID) error {
	res, err := s.resumeRepo.Get(ctx, resumeID, userID)
	if err != nil {
		return err
	}

	if err := s.deleteResumeStorage(ctx, res); err != nil {
		return err
	}

	return s.resumeRepo.Delete(ctx, resumeID, userID)
}

// DeleteAccount removes every object-storage file owned by a user (uploaded
// resumes and generated resume version documents), then deletes the user
// row — cascading sessions, profile, preferences, resumes, tailoring runs,
// applications, and every other user-owned row in the database.
func (s *Service) DeleteAccount(ctx context.Context, userID uuid.UUID) error {
	resumes, err := s.resumeRepo.ListForUser(ctx, userID)
	if err != nil {
		return err
	}

	for _, res := range resumes {
		if err := s.deleteResumeStorage(ctx, res); err != nil {
			return err
		}
	}

	return s.usersRepo.Delete(ctx, userID)
}

// deleteResumeStorage removes a resume's uploaded file and every generated
// PDF/DOCX version file for it. Missing-object errors are ignored (best
// effort — a file that's already gone shouldn't block account/resume
// deletion), but other storage errors are surfaced.
func (s *Service) deleteResumeStorage(ctx context.Context, res resume.Resume) error {
	if res.StorageKey != "" {
		if err := s.storageClient.Delete(ctx, res.StorageKey); err != nil {
			return err
		}
	}

	versions, err := s.resumeVersionRepo.ListForResume(ctx, res.ID)
	if err != nil {
		return err
	}
	for _, v := range versions {
		if v.PDFStorageKey != nil {
			if err := s.storageClient.Delete(ctx, *v.PDFStorageKey); err != nil {
				return err
			}
		}
		if v.DocxStorageKey != nil {
			if err := s.storageClient.Delete(ctx, *v.DocxStorageKey); err != nil {
				return err
			}
		}
	}
	return nil
}
