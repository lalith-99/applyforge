package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/background"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobrequirements"
)

// JobTypeEnrich is the background job type enqueued once per ingested job
// (see Ingest), so JD parsing happens once when a job arrives instead of
// lazily on a user's first view (GetOrParse still guards re-parsing by
// content_hash, so this is safe to enqueue redundantly on every poll).
const JobTypeEnrich = "enrich_job"

// EnrichPayload is the JSON payload enqueued for an enrich_job job.
type EnrichPayload struct {
	JobID string `json:"job_id"`
}

// EnrichWorker processes enrich_job background jobs: it loads the canonical
// job and asks jobrequirements to parse (or reuse a cached parse of) its
// current content.
type EnrichWorker struct {
	repo         *Repository
	requirements *jobrequirements.Service
}

// NewEnrichWorker builds an EnrichWorker.
func NewEnrichWorker(repo *Repository, requirements *jobrequirements.Service) *EnrichWorker {
	return &EnrichWorker{repo: repo, requirements: requirements}
}

// Handle implements background.Handler for JobTypeEnrich.
func (w *EnrichWorker) Handle(ctx context.Context, job background.Job) error {
	var payload EnrichPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}

	jobID, err := uuid.Parse(payload.JobID)
	if err != nil {
		return fmt.Errorf("invalid job id: %w", err)
	}

	j, err := w.repo.GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("load job: %w", err)
	}

	if _, err := w.requirements.GetOrParse(ctx, j.ID, j.Title, j.Description, j.ContentHash); err != nil {
		return fmt.Errorf("parse job requirements: %w", err)
	}
	return nil
}
