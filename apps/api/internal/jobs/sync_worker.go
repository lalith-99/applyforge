package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/background"
)

// JobTypeSyncSource is the background job type enqueued per job source by
// EnqueueSyncTasks.
const JobTypeSyncSource = "sync_job_source"

// SyncSourcePayload is the JSON payload enqueued per job source by
// EnqueueSyncTasks.
type SyncSourcePayload struct {
	JobSourceID string `json:"job_source_id"`
}

// SyncSourceWorker processes sync_job_source background jobs: it re-fetches
// the source's current config (so enable/disable/token edits take effect
// without redeploying) and runs one ingestion poll against it.
type SyncSourceWorker struct {
	repo             *Repository
	ingestion        *IngestionService
	onCatalogChanged func(context.Context)
}

// NewSyncSourceWorker builds a SyncSourceWorker.
func NewSyncSourceWorker(repo *Repository, ingestion *IngestionService) *SyncSourceWorker {
	return &SyncSourceWorker{repo: repo, ingestion: ingestion}
}

// SetOnCatalogChanged registers a best-effort callback invoked when a source
// poll materially changes the catalog. Callers should debounce expensive work
// because many sources may finish close together.
func (w *SyncSourceWorker) SetOnCatalogChanged(fn func(context.Context)) *SyncSourceWorker {
	w.onCatalogChanged = fn
	return w
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
	enabled, err := w.repo.JobSourceEnabled(ctx, cfg.ID)
	if err != nil {
		return fmt.Errorf("check job source state: %w", err)
	}
	if !enabled {
		// A previous queued attempt may have quarantined or disabled this source
		// after this job was enqueued. Treat the stale queue item as complete.
		return nil
	}

	startedAt := time.Now().UTC()
	source, sourceName, err := BuildSource(cfg)
	if err != nil {
		_ = w.repo.TouchJobSource(ctx, cfg.ID, err)
		w.recordPoll(ctx, cfg, startedAt, IngestResult{}, err)
		return fmt.Errorf("build source: %w", err)
	}

	result, ingestErr := w.ingestion.Ingest(ctx, sourceName, source, cfg.CompanyID, cfg.CompanyName)
	if ingestErr != nil && isPermanentSourcePollFailure(cfg.SourceType, ingestErr) {
		w.recordPoll(ctx, cfg, startedAt, result, ingestErr)
		if quarantineErr := w.repo.QuarantineJobSource(ctx, cfg, ingestErr); quarantineErr != nil {
			return fmt.Errorf("quarantine invalid %s source %s: %w", sourceName, cfg.BoardToken, quarantineErr)
		}
		slog.Warn("quarantined permanently invalid job source",
			"job_source_id", cfg.ID,
			"source", sourceName,
			"board_token", cfg.BoardToken,
			"company", cfg.CompanyName,
			"error", ingestErr,
		)
		// Quarantining is a successful terminal handling decision for this queue
		// attempt. Returning nil prevents the background queue from retrying the
		// same known-dead board three times.
		return nil
	}

	if touchErr := w.repo.TouchJobSource(ctx, cfg.ID, ingestErr); touchErr != nil {
		slog.Error("touch job source failed", "job_source_id", cfg.ID, "error", touchErr)
	}
	w.recordPoll(ctx, cfg, startedAt, result, ingestErr)
	if ingestErr != nil {
		return fmt.Errorf("ingest %s (%s): %w", sourceName, cfg.BoardToken, ingestErr)
	}

	slog.Info("job source ingestion completed", "source", sourceName, "board_token", cfg.BoardToken,
		"fetched", result.Fetched, "inserted", result.Inserted, "updated", result.Updated, "deduped", result.Deduped, "closed", result.Closed)
	if w.onCatalogChanged != nil && (result.Inserted > 0 || result.Closed > 0) {
		w.onCatalogChanged(ctx)
	}
	return nil
}

func (w *SyncSourceWorker) recordPoll(ctx context.Context, cfg JobSourceConfig, startedAt time.Time, result IngestResult, pollErr error) {
	outcome := SourcePollOutcome{
		JobSourceID: cfg.ID,
		SourceType:  cfg.SourceType,
		BoardToken:  cfg.BoardToken,
		CompanyName: cfg.CompanyName,
		StartedAt:   startedAt,
		CompletedAt: time.Now().UTC(),
		Result:      result,
		Err:         pollErr,
	}
	if err := w.repo.RecordSourcePoll(ctx, outcome); err != nil && !errors.Is(err, ErrSourceHealthUnavailable) {
		slog.Error("record job source poll failed", "job_source_id", cfg.ID, "error", err)
	}
}
