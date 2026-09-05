package resumeversion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/matching"
	"github.com/lalithlochan/applyforge/apps/api/internal/resume"
	"github.com/lalithlochan/applyforge/apps/api/internal/storage"
	"github.com/lalithlochan/applyforge/apps/api/internal/tailoring"
)

// ErrResumeNotParsed is returned when a caller tries to generate a version
// from a resume that hasn't finished parsing yet.
var ErrResumeNotParsed = errors.New("resume has not finished parsing")

// ErrUnknownFormat is returned for an unsupported download format.
var ErrUnknownFormat = errors.New("unknown document format")

const (
	pdfContentType  = "application/pdf"
	docxContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
)

// Service orchestrates merging approved tailoring suggestions onto a base
// resume and generating downloadable PDF/DOCX documents.
type Service struct {
	repo          *Repository
	resumeRepo    *resume.Repository
	tailoringRepo *tailoring.Repository
	aiClient      *aiclient.Client
	storageClient *storage.Client
	matchingSvc   *matching.Service
}

// NewService builds a Service.
func NewService(
	repo *Repository,
	resumeRepo *resume.Repository,
	tailoringRepo *tailoring.Repository,
	aiClient *aiclient.Client,
	storageClient *storage.Client,
	matchingSvc *matching.Service,
) *Service {
	return &Service{
		repo:          repo,
		resumeRepo:    resumeRepo,
		tailoringRepo: tailoringRepo,
		aiClient:      aiClient,
		storageClient: storageClient,
		matchingSvc:   matchingSvc,
	}
}

// GenerateVersion merges any approved/edited tailoring suggestions for the
// given run onto the base resume's parsed profile, persists a new version,
// and generates + stores PDF/DOCX documents for it.
func (s *Service) GenerateVersion(ctx context.Context, userID, resumeID uuid.UUID, jobID, tailoringRunID *uuid.UUID) (Version, error) {
	baseResume, err := s.resumeRepo.Get(ctx, resumeID, userID)
	if err != nil {
		return Version{}, err
	}
	if baseResume.Status != resume.StatusParsed {
		return Version{}, ErrResumeNotParsed
	}

	var base aiclient.ResumeProfile
	if err := json.Unmarshal(baseResume.ParsedProfile, &base); err != nil {
		return Version{}, fmt.Errorf("decode parsed profile: %w", err)
	}

	var suggestions []tailoring.Suggestion
	var alignmentScore *int32
	var mode *string
	if tailoringRunID != nil {
		run, err := s.tailoringRepo.GetRunForUser(ctx, *tailoringRunID, userID)
		if err != nil {
			return Version{}, err
		}
		alignmentScore = run.AlignmentScoreAfter
		runMode := run.Mode
		mode = &runMode

		suggestions, err = s.tailoringRepo.ListSuggestions(ctx, *tailoringRunID)
		if err != nil {
			return Version{}, err
		}
	}

	merged := mergeContent(base, suggestions)

	var matchScore *int32
	if jobID != nil {
		result, err := s.matchingSvc.Match(ctx, *jobID, userID)
		if err != nil {
			return Version{}, err
		}
		score := int32(result.TotalScore)
		matchScore = &score
	}

	versionNumber, err := s.repo.NextVersionNumber(ctx, resumeID)
	if err != nil {
		return Version{}, err
	}

	version, err := s.repo.Create(ctx, userID, resumeID, jobID, tailoringRunID, versionNumber, merged, matchScore, alignmentScore, mode)
	if err != nil {
		return Version{}, err
	}

	pdfBytes, err := s.aiClient.GeneratePDF(ctx, merged)
	if err != nil {
		return Version{}, fmt.Errorf("generate pdf: %w", err)
	}
	docxBytes, err := s.aiClient.GenerateDOCX(ctx, merged)
	if err != nil {
		return Version{}, fmt.Errorf("generate docx: %w", err)
	}

	pdfKey := fmt.Sprintf("resume-versions/%s/resume.pdf", version.ID)
	docxKey := fmt.Sprintf("resume-versions/%s/resume.docx", version.ID)

	if err := s.storageClient.Put(ctx, pdfKey, bytes.NewReader(pdfBytes), int64(len(pdfBytes)), pdfContentType); err != nil {
		return Version{}, fmt.Errorf("store pdf: %w", err)
	}
	if err := s.storageClient.Put(ctx, docxKey, bytes.NewReader(docxBytes), int64(len(docxBytes)), docxContentType); err != nil {
		return Version{}, fmt.Errorf("store docx: %w", err)
	}

	return s.repo.SetDocuments(ctx, version.ID, pdfKey, docxKey)
}

// Download fetches the generated document bytes for a version, along with
// the content type and a suggested filename.
func (s *Service) Download(ctx context.Context, versionID, userID uuid.UUID, format string) ([]byte, string, string, error) {
	version, err := s.repo.GetForUser(ctx, versionID, userID)
	if err != nil {
		return nil, "", "", err
	}

	var key, contentType, ext string
	switch format {
	case "pdf":
		if version.PDFStorageKey == nil {
			return nil, "", "", ErrNotFound
		}
		key, contentType, ext = *version.PDFStorageKey, pdfContentType, "pdf"
	case "docx":
		if version.DocxStorageKey == nil {
			return nil, "", "", ErrNotFound
		}
		key, contentType, ext = *version.DocxStorageKey, docxContentType, "docx"
	default:
		return nil, "", "", ErrUnknownFormat
	}

	data, err := s.storageClient.Get(ctx, key)
	if err != nil {
		return nil, "", "", err
	}

	filename := fmt.Sprintf("resume-v%d.%s", version.VersionNumber, ext)
	return data, contentType, filename, nil
}
