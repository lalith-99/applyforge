package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/background"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobrequirements"
)

// JobTypeEnrich is the background job type enqueued once per ingested job
// (see Ingest), so JD parsing happens once when a job arrives instead of
// lazily on a user's first view (GetOrParse still guards re-parsing by
// content_hash, so this is safe to enqueue redundantly on every poll).
const JobTypeEnrich = "enrich_job"

const (
	eagerEnrichmentMaxAge       = 24 * time.Hour
	eagerEnrichmentFutureSkew   = 5 * time.Minute
)

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

	// Eager parsing is an optimization for the recommendation funnel, not a
	// correctness requirement. Avoid spending AI/JD-parsing work on jobs that
	// cannot enter the strict fresh-US-software shortlist. If a skipped job is
	// opened or matched explicitly later, GetOrParse still parses it lazily.
	if !shouldEagerEnrich(j, time.Now().UTC()) {
		return nil
	}

	if _, err := w.requirements.GetOrParse(ctx, j.ID, j.Title, j.Description, j.ContentHash); err != nil {
		return fmt.Errorf("parse job requirements: %w", err)
	}
	return nil
}

func shouldEagerEnrich(j Job, now time.Time) bool {
	if j.CanonicalJobID != nil {
		return false
	}
	if status := strings.TrimSpace(j.Status); status != "" && !strings.EqualFold(status, "ACTIVE") {
		return false
	}
	if j.CountryCode != nil {
		countryCode := strings.TrimSpace(*j.CountryCode)
		if countryCode != "" && !strings.EqualFold(countryCode, "US") {
			return false
		}
	}
	if j.PostedAt == nil {
		return false
	}

	now = now.UTC()
	postedAt := j.PostedAt.UTC()
	if postedAt.Before(now.Add(-eagerEnrichmentMaxAge)) {
		return false
	}
	if postedAt.After(now.Add(eagerEnrichmentFutureSkew)) {
		return false
	}

	return classifyTitle(j.Title).Family != "EXCLUDED"
}
