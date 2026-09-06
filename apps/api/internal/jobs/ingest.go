package jobs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/background"
)

// IngestionService fetches from a JobSource and idempotently upserts results
// into the canonical jobs table (see MASTER_REQUIREMENTS.md §49: normalize
// -> deduplicate, before any scoring/AI work happens).
type IngestionService struct {
	repo  *Repository
	queue *background.Queue // nil in tests that don't exercise async enqueueing
}

// NewIngestionService builds an IngestionService. queue may be nil (e.g. in
// unit tests that only exercise Ingest's upsert logic directly), in which
// case async source-sync/enrichment enqueueing is simply skipped.
func NewIngestionService(repo *Repository, queue *background.Queue) *IngestionService {
	return &IngestionService{repo: repo, queue: queue}
}

// IngestResult summarizes the outcome of a single source poll.
type IngestResult struct {
	Fetched  int
	Inserted int
	Updated  int
}

// Ingest fetches all postings from source and upserts them, attributing them
// to the given company by default (companies are configured via job_sources,
// since most board APIs don't include a company display name in their
// payload). Aggregator sources (e.g. Arbeitnow) that DO return a per-job
// company name via raw.CompanyName have that company dynamically
// upserted/reused instead, so a single job_sources row can ingest postings
// from many different companies.
func (s *IngestionService) Ingest(ctx context.Context, sourceName string, source JobSource, companyID uuid.UUID, companyName string) (IngestResult, error) {
	rawJobs, _, err := source.Fetch(ctx, nil)
	if err != nil {
		return IngestResult{}, fmt.Errorf("fetch from %s: %w", sourceName, err)
	}

	// Cache resolved companies within this poll to avoid re-upserting the
	// same company for every one of its postings.
	resolvedCompanies := map[string]uuid.UUID{}

	result := IngestResult{Fetched: len(rawJobs)}
	for _, raw := range rawJobs {
		jobCompanyID, jobCompanyName := companyID, companyName
		if raw.CompanyName != "" {
			normalized := normalizeCompanyName(raw.CompanyName)
			if id, ok := resolvedCompanies[normalized]; ok {
				jobCompanyID = id
			} else if id, err := s.repo.UpsertCompany(ctx, raw.CompanyName, normalized); err == nil {
				jobCompanyID = id
				resolvedCompanies[normalized] = id
			} else {
				slog.Error("upsert company failed, falling back to source company", "source", sourceName, "company_name", raw.CompanyName, "error", err)
			}
			jobCompanyName = raw.CompanyName
		}

		job := Job{
			Source:          sourceName,
			ExternalID:      raw.ExternalID,
			CompanyID:       jobCompanyID,
			CompanyName:     jobCompanyName,
			Title:           raw.Title,
			NormalizedTitle: normalizeTitle(raw.Title),
			Description:     raw.Description,
			LocationText:    strOrNil(raw.LocationText),
			RemoteType:      strOrNil(raw.RemoteType),
			EmploymentType:  strOrNil(raw.EmploymentType),
			ApplyURL:        strOrNil(raw.ApplyURL),
			SourceURL:       strOrNil(raw.SourceURL),
			PostedAt:        raw.PostedAt,
			ContentHash:     contentHash(jobCompanyName, raw.Title, raw.LocationText, raw.Description),
		}

		upserted, err := s.repo.UpsertJob(ctx, job)
		if err != nil {
			slog.Error("upsert job failed", "source", sourceName, "external_id", raw.ExternalID, "error", err)
			continue
		}

		// Enrich eagerly only for jobs seen for the first time. Re-touching
		// an already-seen job on every poll (the common case once a source
		// is caught up) would otherwise re-enqueue enrichment for its whole
		// backlog every cycle; GetOrParse's content_hash cache means that's
		// merely wasteful for jobs that already have a cached parse, but for
		// a source's FIRST poll after enabling eager enrichment it means one
		// real (paid) AI call per already-ingested job, all at once. A JD
		// that's edited after first ingestion is still picked up lazily via
		// GetOrParse's content_hash check on next view, same as before this
		// change - just not proactively.
		if upserted.Inserted {
			result.Inserted++
			if s.queue != nil {
				payload := EnrichPayload{JobID: upserted.Job.ID.String()}
				if err := s.queue.Enqueue(ctx, JobTypeEnrich, payload, 3); err != nil {
					slog.Error("enqueue enrich_job failed", "job_id", upserted.Job.ID, "error", err)
				}
			}
		} else {
			result.Updated++
		}
	}

	return result, nil
}

// BuildSource constructs a JobSource for a stored job_sources configuration row.
func BuildSource(cfg JobSourceConfig) (JobSource, string, error) {
	switch cfg.SourceType {
	case "GREENHOUSE":
		return NewGreenhouseSource(cfg.BoardToken), "GREENHOUSE", nil
	case "LEVER":
		return NewLeverSource(cfg.BoardToken), "LEVER", nil
	case "ASHBY":
		return NewAshbySource(cfg.BoardToken), "ASHBY", nil
	case "ARBEITNOW":
		return NewArbeitnowSource(), "ARBEITNOW", nil
	default:
		return nil, "", fmt.Errorf("unknown source type: %s", cfg.SourceType)
	}
}

// SyncAll polls every enabled job source once, synchronously, in-process.
// Kept for tests and for the one-shot admin trigger; production scheduling
// uses EnqueueSyncTasks instead so polling many sources doesn't block on a
// single slow/rate-limited provider (see internal/jobs/sync_worker.go).
func (s *IngestionService) SyncAll(ctx context.Context) error {
	sources, err := s.repo.ListJobSources(ctx)
	if err != nil {
		return err
	}

	for _, cfg := range sources {
		source, sourceName, err := BuildSource(cfg)
		if err != nil {
			slog.Error("unknown job source type", "source_type", cfg.SourceType, "error", err)
			continue
		}

		result, ingestErr := s.Ingest(ctx, sourceName, source, cfg.CompanyID, cfg.CompanyName)
		if touchErr := s.repo.TouchJobSource(ctx, cfg.ID, ingestErr); touchErr != nil {
			slog.Error("touch job source failed", "job_source_id", cfg.ID, "error", touchErr)
		}
		if ingestErr != nil {
			slog.Error("job source ingestion failed", "source", sourceName, "board_token", cfg.BoardToken, "error", ingestErr)
			continue
		}
		slog.Info("job source ingestion completed", "source", sourceName, "board_token", cfg.BoardToken,
			"fetched", result.Fetched, "inserted", result.Inserted, "updated", result.Updated)
	}
	return nil
}

// EnqueueSyncTasks enqueues one sync_job_source background task per enabled
// job source instead of polling them sequentially in-process. Multiple
// worker processes/goroutines can then claim and fetch from providers
// concurrently (see SyncSourceWorker), so a slow or rate-limited source
// doesn't delay every other source's poll.
func (s *IngestionService) EnqueueSyncTasks(ctx context.Context) error {
	if s.queue == nil {
		return fmt.Errorf("ingestion service has no queue configured")
	}

	sources, err := s.repo.ListJobSources(ctx)
	if err != nil {
		return err
	}

	for _, cfg := range sources {
		payload := SyncSourcePayload{JobSourceID: cfg.ID.String()}
		if err := s.queue.Enqueue(ctx, JobTypeSyncSource, payload, 3); err != nil {
			slog.Error("enqueue sync_job_source failed", "job_source_id", cfg.ID, "board_token", cfg.BoardToken, "error", err)
		}
	}
	return nil
}

func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
