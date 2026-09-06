package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/background"
)

// JobTypeSyncSource is the background job type enqueued per job source by
// EnqueueSyncTasks.
const JobTypeSyncSource = "sync_job_source"

// SyncSourcePayload is the JSON payload enqueued for a sync_job_source job.
type SyncSourcePayload struct {
	JobSourceID string `json:"job_source_id"`
}

// SyncSourceWorker processes sync_job_source background jobs: it re-fetches
// the source's current config (so enable/disable/token edits take effect
// without redeploying) and runs one ingestion poll against it.
type SyncSourceWorker struct {
	repo      *Repository
	ingestion *IngestionService
}

// NewSyncSourceWorker builds a SyncSourceWorker.
func NewSyncSourceWorker(repo *Repository, ingestion *IngestionService) *SyncSourceWorker {
	return &SyncSourceWorker{repo: repo, ingestion: ingestion}
}

// Handle implements background.Handler for JobTypeSyncSource.
func (w *SyncSourceWorker) Handle(ctx context.Context, job background.Job) error {
	var payload SyncSourcePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}

	jobSourceID, err := uuid.Parse(payload.JobSourceID)
	if err != nil {
		return fmt.Errorf("invalid job source id: %w", err)
	}

	cfg, err := w.repo.GetJobSourceByID(ctx, jobSourceID)
	if err != nil {
		return fmt.Errorf("load job source: %w", err)
	}

	source, sourceName, err := BuildSource(cfg)
	if err != nil {
		return fmt.Errorf("build source: %w", err)
	}

	result, ingestErr := w.ingestion.Ingest(ctx, sourceName, source, cfg.CompanyID, cfg.CompanyName)
	if touchErr := w.repo.TouchJobSource(ctx, cfg.ID, ingestErr); touchErr != nil {
		slog.Error("touch job source failed", "job_source_id", cfg.ID, "error", touchErr)
	}
	if ingestErr != nil {
		return fmt.Errorf("ingest %s (%s): %w", sourceName, cfg.BoardToken, ingestErr)
	}

	slog.Info("job source ingestion completed", "source", sourceName, "board_token", cfg.BoardToken,
		"fetched", result.Fetched, "inserted", result.Inserted, "updated", result.Updated, "deduped", result.Deduped, "closed", result.Closed)
	return nil
}
