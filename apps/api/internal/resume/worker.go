package resume

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/background"
	"github.com/lalithlochan/applyforge/apps/api/internal/candidateskills"
	"github.com/lalithlochan/applyforge/apps/api/internal/skills"
	"github.com/lalithlochan/applyforge/apps/api/internal/storage"
)

// JobTypeParse is the background job type enqueued after a resume upload.
const JobTypeParse = "parse_resume"

// ParsePayload is the JSON payload enqueued for a parse_resume job.
type ParsePayload struct {
	ResumeID string `json:"resume_id"`
}

// ParseWorker processes parse_resume background jobs: it downloads the
// uploaded file, extracts text and a structured profile via the AI worker,
// and materializes candidate skills from the result.
type ParseWorker struct {
	resumes    *Repository
	skillsRepo *candidateskills.Repository
	normalizer *skills.Normalizer
	storage    *storage.Client
	aiClient   *aiclient.Client
	onParsed   func(ctx context.Context, userID uuid.UUID) // optional; see SetOnParsed
}

// NewParseWorker builds a ParseWorker.
func NewParseWorker(resumes *Repository, skillsRepo *candidateskills.Repository, normalizer *skills.Normalizer, storageClient *storage.Client, aiClient *aiclient.Client) *ParseWorker {
	return &ParseWorker{
		resumes:    resumes,
		skillsRepo: skillsRepo,
		normalizer: normalizer,
		storage:    storageClient,
		aiClient:   aiClient,
	}
}

// SetOnParsed registers a callback invoked after a resume is successfully
// parsed (e.g. to enqueue a CandidateIntelligenceProfile rebuild - kept as a
// callback rather than a direct import so package resume doesn't need to
// know about candidateprofile).
func (w *ParseWorker) SetOnParsed(fn func(ctx context.Context, userID uuid.UUID)) {
	w.onParsed = fn
}

// Handle implements background.Handler for JobTypeParse.
func (w *ParseWorker) Handle(ctx context.Context, job background.Job) error {
	var payload ParsePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}

	resumeID, err := uuid.Parse(payload.ResumeID)
	if err != nil {
		return fmt.Errorf("invalid resume id: %w", err)
	}

	if err := w.process(ctx, resumeID); err != nil {
		if markErr := w.resumes.MarkFailed(ctx, resumeID, err.Error()); markErr != nil {
			return fmt.Errorf("process failed (%v) and mark-failed also failed: %w", err, markErr)
		}
		return err
	}
	return nil
}

func (w *ParseWorker) process(ctx context.Context, resumeID uuid.UUID) error {
	if err := w.resumes.MarkParsing(ctx, resumeID); err != nil {
		return fmt.Errorf("mark parsing: %w", err)
	}

	r, err := w.resumes.GetByID(ctx, resumeID)
	if err != nil {
		return fmt.Errorf("load resume: %w", err)
	}

	fileBytes, err := w.storage.Get(ctx, r.StorageKey)
	if err != nil {
		return fmt.Errorf("download resume file: %w", err)
	}

	rawText, err := w.aiClient.ExtractResumeText(ctx, r.OriginalFilename, r.MimeType, fileBytes)
	if err != nil {
		return fmt.Errorf("extract text: %w", err)
	}

	profile, err := w.aiClient.ParseResume(ctx, rawText)
	if err != nil {
		return fmt.Errorf("parse resume: %w", err)
	}

	profileJSON, err := json.Marshal(profile)
	if err != nil {
		return fmt.Errorf("marshal profile: %w", err)
	}

	if err := w.resumes.MarkParsed(ctx, resumeID, rawText, profileJSON); err != nil {
		return fmt.Errorf("mark parsed: %w", err)
	}

	experiences := make([]Experience, 0, len(profile.Experiences))
	for _, e := range profile.Experiences {
		experiences = append(experiences, Experience{
			Company:        e.Company,
			Title:          e.Title,
			StartDate:      e.StartDate,
			EndDate:        e.EndDate,
			Location:       e.Location,
			Bullets:        e.Bullets,
			DetectedSkills: e.DetectedSkills,
			Technologies:   e.Technologies,
		})
	}
	if err := w.resumes.ReplaceExperiences(ctx, resumeID, experiences); err != nil {
		return fmt.Errorf("store experiences: %w", err)
	}

	if err := w.upsertCandidateSkills(ctx, r.UserID, profile); err != nil {
		return fmt.Errorf("upsert candidate skills: %w", err)
	}

	if w.onParsed != nil {
		w.onParsed(ctx, r.UserID)
	}

	return nil
}

func (w *ParseWorker) upsertCandidateSkills(ctx context.Context, userID uuid.UUID, profile aiclient.ResumeProfile) error {
	seen := map[string]bool{}
	upsert := func(raw string) error {
		display := w.normalizer.Canonical(raw)
		key := skills.NormalizedKey(display)
		if key == "" || seen[key] {
			return nil
		}
		seen[key] = true
		_, err := w.skillsRepo.Upsert(ctx, userID, key, display, nil, nil,
			candidateskills.SourceMasterResume, candidateskills.StatusVerifiedProfessional)
		return err
	}

	for _, s := range profile.Skills {
		if err := upsert(s); err != nil {
			return err
		}
	}
	for _, exp := range profile.Experiences {
		for _, s := range exp.DetectedSkills {
			if err := upsert(s); err != nil {
				return err
			}
		}
		for _, s := range exp.Technologies {
			if err := upsert(s); err != nil {
				return err
			}
		}
	}
	return nil
}
