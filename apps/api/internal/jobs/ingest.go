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
	case "ARBEITNOW":
		return NewArbeitnowSource(), "ARBEITNOW", nil
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
