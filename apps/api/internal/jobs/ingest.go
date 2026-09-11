package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/background"
)

// IngestionService fetches from a JobSource and idempotently upserts results
// into the canonical jobs table (see MASTER_REQUIREMENTS.md §49: normalize
// -> deduplicate, before any scoring/AI work happens).
type IngestionService struct {
	repo              *Repository
	queue             *background.Queue // nil in tests that don't exercise async enqueueing
	embeddingsEnabled bool
}

// NewIngestionService builds an IngestionService. queue may be nil (e.g. in
// unit tests that only exercise Ingest's upsert logic directly), in which
// case async source-sync/enrichment enqueueing is simply skipped.
func NewIngestionService(repo *Repository, queue *background.Queue) *IngestionService {
	return &IngestionService{repo: repo, queue: queue, embeddingsEnabled: true}
}

func (s *IngestionService) WithEmbeddingsEnabled(enabled bool) *IngestionService {
	s.embeddingsEnabled = enabled
	return s
}

// IngestResult summarizes the outcome of a single source poll.
type IngestResult struct {
	Fetched  int
	Inserted int
	Updated  int
	Deduped  int   // new rows linked to an existing canonical job from a different source (see FindCanonicalByFingerprint)
	Closed   int64 // jobs marked CLOSED for no longer appearing in this poll (0 for aggregator sources, see CloseStaleJobs)
}

// Ingest fetches all postings from source and upserts them, attributing them
// to the given company by default (companies are configured via job_sources,
// since most board APIs don't include a company display name in their
// payload). Aggregator sources (e.g. Arbeitnow) that DO return a per-job
// company name via raw.CompanyName have that company dynamically
// resolved/reused instead, so a single job_sources row can ingest postings
// from many different companies.
func (s *IngestionService) Ingest(ctx context.Context, sourceName string, source JobSource, companyID uuid.UUID, companyName string) (IngestResult, error) {
	pollStart := time.Now()
	rawJobs, _, err := source.Fetch(ctx, nil)
	if err != nil {
		return IngestResult{}, fmt.Errorf("fetch from %s: %w", sourceName, err)
	}

	if completeSnapshot, ok := source.(CompleteSnapshotSource); ok {
		if err := s.repo.TouchSeenJobs(ctx, sourceName, companyID, completeSnapshot.SeenExternalIDs(), pollStart); err != nil {
			slog.Error("touch complete source snapshot failed",
				"source", sourceName,
				"company_id", companyID,
				"error", err,
			)
		}
	}

	// Cache name-only company resolutions within this poll. Jobs that expose a
	// direct ATS source are resolved through the source graph first because that
	// identity evidence is stronger than the provider's display-name spelling.
	type resolvedCompany struct {
		id   uuid.UUID
		name string
	}
	resolvedCompanies := map[string]resolvedCompany{}

	result := IngestResult{Fetched: len(rawJobs)}
	for _, raw := range rawJobs {
		jobCompanyID, jobCompanyName := companyID, companyName
		discoveries := DetectCompanySources(raw.ApplyURL, raw.SourceURL)

		if raw.CompanyName != "" {
			resolvedBySource := false
			if len(discoveries) > 0 {
				resolvedID, resolvedName, ok, resolveErr := s.repo.ResolveCompanyByDiscoveredSources(ctx, discoveries)
				if resolveErr != nil {
					slog.Error("resolve company from discovered source failed",
						"source", sourceName,
						"company_name", raw.CompanyName,
						"error", resolveErr,
					)
				} else if ok {
					jobCompanyID = resolvedID
					jobCompanyName = resolvedName
					resolvedBySource = true
				}
			}

			if !resolvedBySource {
				normalized := normalizeCompanyName(raw.CompanyName)
				if cached, ok := resolvedCompanies[normalized]; ok {
					jobCompanyID = cached.id
					jobCompanyName = cached.name
				} else if id, err := s.repo.UpsertCompany(ctx, raw.CompanyName, normalized); err == nil {
					jobCompanyID = id
					jobCompanyName = raw.CompanyName
					resolvedCompanies[normalized] = resolvedCompany{id: id, name: raw.CompanyName}
				} else {
					slog.Error("upsert company failed, falling back to source company", "source", sourceName, "company_name", raw.CompanyName, "error", err)
					jobCompanyName = raw.CompanyName
				}
			}
		}

		location := normalizeLocation(raw)
		classification := classifyTitle(raw.Title)
		country := raw.Country
		if country == "" && location.CountryCode == "US" {
			country = "United States"
		}
		state := firstNonEmpty(raw.State, location.StateCode)
		job := Job{
			Source:               sourceName,
			ExternalID:           raw.ExternalID,
			CompanyID:            jobCompanyID,
			CompanyName:          jobCompanyName,
			Title:                raw.Title,
			NormalizedTitle:      normalizeTitle(raw.Title),
			Description:          raw.Description,
			Country:              strOrNil(country),
			State:                strOrNil(state),
			City:                 strOrNil(firstNonEmpty(raw.City, location.City)),
			LocationText:         strOrNil(raw.LocationText),
			CountryCode:          strOrNil(location.CountryCode),
			StateCode:            strOrNil(location.StateCode),
			WorkplaceType:        location.WorkplaceType,
			RemoteScope:          location.RemoteScope,
			EligibleCountryCodes: location.EligibleCountryCodes,
			LocationConfidence:   location.LocationConfidence,
			RemoteType:           strOrNil(raw.RemoteType),
			EmploymentType:       strOrNil(normalizeEmploymentType(raw.EmploymentType)),
			ApplyURL:             strOrNil(raw.ApplyURL),
			SourceURL:            strOrNil(raw.SourceURL),
			PostedAt:             raw.PostedAt,
			ContentHash:          contentHash(jobCompanyName, raw.Title, raw.LocationText, raw.Description),
			Fingerprint:          buildFingerprint(jobCompanyName, raw.Title, raw.LocationText, raw.Description),
		}

		upserted, err := s.repo.UpsertJob(ctx, job)
		if err != nil {
			slog.Error("upsert job failed", "source", sourceName, "external_id", raw.ExternalID, "error", err)
			continue
		}

		if err := s.repo.UpdateExplicitSponsorshipDenied(
			ctx,
			upserted.Job.ID,
			explicitSponsorshipDenied(raw.Description),
		); err != nil {
			slog.Error("update sponsorship prefilter failed", "job_id", upserted.Job.ID, "error", err)
		}

		// Aggregated discovery sources frequently carry the employer's direct
		// application URL. Learn supported ATS board tokens from those URLs and
		// promote the canonical company to direct polling automatically. This is
		// best-effort: a source-registry failure must never discard the job itself.
		if len(discoveries) > 0 {
			if err := s.repo.RecordDiscoveredCompanySources(ctx, jobCompanyID, discoveries); err != nil {
				slog.Error("record discovered company source failed",
					"source", sourceName,
					"company_id", jobCompanyID,
					"company_name", jobCompanyName,
					"error", err,
				)
			}
		}
		// Cross-source dedupe: a brand new posting whose fingerprint matches
		// one already canonical from a DIFFERENT (source, external_id) is
		// linked to it instead of surfacing as a separate result. Only
		// relevant for genuinely new rows - an existing job being re-touched
		// already has whatever canonical link it was given the first time.
		if upserted.Inserted && job.Fingerprint != "" {
			canonical, findErr := s.repo.FindCanonicalByFingerprint(ctx, job.Fingerprint, upserted.Job.ID)
			if findErr == nil {
				var setErr error
				if sourcePriority(upserted.Job.Source) > sourcePriority(canonical.Source) {
					setErr = s.repo.PromoteCanonicalJob(ctx, upserted.Job.ID, canonical.ID)
				} else {
					setErr = s.repo.SetCanonicalJobID(ctx, upserted.Job.ID, canonical.ID)
				}
				if setErr != nil {
					slog.Error("set canonical job id failed", "job_id", upserted.Job.ID, "canonical_job_id", canonical.ID, "error", setErr)
				} else {
					result.Deduped++
				}
			} else if !errors.Is(findErr, ErrNoCanonicalMatch) {
				slog.Error("find canonical by fingerprint failed", "job_id", upserted.Job.ID, "error", findErr)
			}
		}

		if upserted.Inserted {
			result.Inserted++
		} else {
			result.Updated++
		}

		// Eager AI runs only for a genuinely new posting or a posting whose
		// content hash changed. Hourly re-touches of unchanged jobs therefore
		// remain free, while edited descriptions get fresh requirements and
		// embeddings without waiting for a user to open the job.
		shouldRefreshAI := upserted.Inserted || upserted.ContentChanged
		if shouldRefreshAI && s.queue != nil &&
			location.CountryCode == "US" &&
			strings.TrimSpace(raw.Description) != "" &&
			isFreshForEagerAI(raw.PostedAt, pollStart) {
			switch classification.Classification {
			case "IC_SOFTWARE":
				payload := EnrichPayload{JobID: upserted.Job.ID.String()}
				if err := s.queue.Enqueue(ctx, JobTypeEnrich, payload, 3); err != nil {
					slog.Error("enqueue enrich_job failed", "job_id", upserted.Job.ID, "error", err)
				}
				if s.embeddingsEnabled {
					if err := s.queue.Enqueue(ctx, JobTypeEmbed, EmbedPayload{JobID: upserted.Job.ID.String()}, 3); err != nil {
						slog.Error("enqueue embed_job failed", "job_id", upserted.Job.ID, "error", err)
					}
				}
			case "UNKNOWN":
				if err := s.queue.Enqueue(ctx, JobTypeClassifyRole, ClassifyRolePayload{JobID: upserted.Job.ID.String()}, 3); err != nil {
					slog.Error("enqueue classify_job_role failed", "job_id", upserted.Job.ID, "error", err)
				}
			}
		}
	}

	// Closure detection: only valid for sources that return their FULL
	// current listing every poll. Arbeitnow's page cap means "not seen this
	// poll" doesn't reliably mean "closed" - see CloseStaleJobs's doc
	// comment - so it's deliberately excluded here.
	if sourceName != "ARBEITNOW" &&
		sourceName != "BRIGHTDATA" &&
		sourceName != "SERPAPI_GOOGLE_JOBS" &&
		sourceName != "CAREER_PAGE" {
		closed, closeErr := s.repo.CloseStaleJobs(ctx, sourceName, companyID, pollStart)
		if closeErr != nil {
			slog.Error("close stale jobs failed", "source", sourceName, "company_id", companyID, "error", closeErr)
		} else {
			result.Closed = closed
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
	case "SMARTRECRUITERS":
		return NewSmartRecruitersSource(cfg.BoardToken), "SMARTRECRUITERS", nil
	case "WORKABLE":
		return NewWorkableSource(cfg.BoardToken), "WORKABLE", nil
	case "WORKDAY":
		source, err := NewWorkdaySource(cfg.BoardToken)
		if err != nil {
			return nil, "", err
		}
		return source, "WORKDAY", nil
	case "ICIMS":
		source, err := NewICIMSSource(cfg.BoardToken)
		if err != nil {
			return nil, "", err
		}
		return source, "ICIMS", nil
	case "CAREER_PAGE":
		source, err := NewCareerPageSource(cfg.BoardToken)
		if err != nil {
			return nil, "", err
		}
		return source, "CAREER_PAGE", nil
	case "ARBEITNOW":
		return NewArbeitnowSource(), "ARBEITNOW", nil
	case "BRIGHTDATA":
		config, err := BrightDataConfigFromEnv()
		if err != nil {
			return nil, "", err
		}
		return NewBrightDataSource(cfg.BoardToken, config), "BRIGHTDATA", nil
	case "SERPAPI_GOOGLE_JOBS":
		config, err := SerpAPIGoogleJobsConfigFromEnv()
		if err != nil {
			return nil, "", err
		}
		return NewSerpAPIGoogleJobsSource(cfg.BoardToken, config), "SERPAPI_GOOGLE_JOBS", nil
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
			"fetched", result.Fetched, "inserted", result.Inserted, "updated", result.Updated, "deduped", result.Deduped, "closed", result.Closed)
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

	if closed, err := s.repo.CloseRetiredManualSourceJobs(ctx); err != nil {
		slog.Error("retired manual source cleanup failed", "error", err)
	} else if closed > 0 {
		slog.Info("closed stale jobs from retired manual sources", "count", closed)
	}

	sources, err := s.repo.ListDueJobSources(ctx)
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

const eagerAIMaxJobAge = 30 * 24 * time.Hour

func isFreshForEagerAI(postedAt *time.Time, now time.Time) bool {
	if postedAt == nil {
		return false
	}
	age := now.Sub(*postedAt)
	// Future timestamps within a small provider-clock skew are still fresh;
	// wildly future timestamps should not consume AI budget.
	return age >= -24*time.Hour && age <= eagerAIMaxJobAge
}

func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func sourcePriority(source string) int {
	switch source {
	case "GREENHOUSE", "LEVER", "ASHBY", "SMARTRECRUITERS", "WORKABLE", "WORKDAY", "ICIMS":
		return 100
	case "CAREER_PAGE":
		return 85
	case "BRIGHTDATA":
		return 60
	case "SERPAPI_GOOGLE_JOBS":
		return 50
	case "ARBEITNOW":
		return 40
	default:
		return 20
	}
}
