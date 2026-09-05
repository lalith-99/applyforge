package jobs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// IngestionService fetches from a JobSource and idempotently upserts results
// into the canonical jobs table (see MASTER_REQUIREMENTS.md §49: normalize
// -> deduplicate, before any scoring/AI work happens).
type IngestionService struct {
	repo *Repository
}

// NewIngestionService builds an IngestionService.
func NewIngestionService(repo *Repository) *IngestionService {
	return &IngestionService{repo: repo}
}

// IngestResult summarizes the outcome of a single source poll.
type IngestResult struct {
	Fetched  int
	Inserted int
	Updated  int
}

// Ingest fetches all postings from source and upserts them, attributing them
// to the given company (companies are configured via job_sources, not
// inferred from connector responses — most board APIs don't include a
// company display name in their payload).
func (s *IngestionService) Ingest(ctx context.Context, sourceName string, source JobSource, companyID uuid.UUID, companyName string) (IngestResult, error) {
	rawJobs, _, err := source.Fetch(ctx, nil)
	if err != nil {
		return IngestResult{}, fmt.Errorf("fetch from %s: %w", sourceName, err)
	}

	result := IngestResult{Fetched: len(rawJobs)}
	for _, raw := range rawJobs {
		job := Job{
			Source:          sourceName,
			ExternalID:      raw.ExternalID,
			CompanyID:       companyID,
			CompanyName:     companyName,
			Title:           raw.Title,
			NormalizedTitle: normalizeTitle(raw.Title),
			Description:     raw.Description,
			LocationText:    strOrNil(raw.LocationText),
			RemoteType:      strOrNil(raw.RemoteType),
			EmploymentType:  strOrNil(raw.EmploymentType),
			ApplyURL:        strOrNil(raw.ApplyURL),
			SourceURL:       strOrNil(raw.SourceURL),
			PostedAt:        raw.PostedAt,
			ContentHash:     contentHash(companyName, raw.Title, raw.LocationText, raw.Description),
		}

		upserted, err := s.repo.UpsertJob(ctx, job)
		if err != nil {
			slog.Error("upsert job failed", "source", sourceName, "external_id", raw.ExternalID, "error", err)
			continue
		}
		if upserted.Inserted {
			result.Inserted++
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
	default:
		return nil, "", fmt.Errorf("unknown source type: %s", cfg.SourceType)
	}
}

// SyncAll polls every enabled job source once.
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

func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
