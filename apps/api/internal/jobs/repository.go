package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// Job is the domain representation of a canonical job posting.
type Job struct {
	ID                   uuid.UUID
	Source               string
	ExternalID           string
	CompanyID            uuid.UUID
	CompanyName          string
	Title                string
	NormalizedTitle      string
	Seniority            *string
	Description          string
	Country              *string
	State                *string
	City                 *string
	LocationText         *string
	CountryCode          *string
	StateCode            *string
	WorkplaceType        string
	RemoteScope          string
	EligibleCountryCodes []string
	LocationConfidence   string
	RemoteType           *string
	EmploymentType       *string
	SalaryMin            *int32
	SalaryMax            *int32
	SalaryCurrency       *string
	ApplyURL             *string
	SourceURL            *string
	PostedAt             *time.Time
	FirstSeenAt          time.Time
	UpdatedAt            time.Time
	LastSeenAt           time.Time
	ContentHash          string
	Fingerprint          string
	CanonicalJobID       *uuid.UUID // set when this row is a cross-source duplicate of another job
	Status               string
	CreatedAt            time.Time
}

// jobFromRow converts a job row into the domain Job type. Takes
// db.GetJobByIDRow specifically, but every other job-selecting query here
// uses an identical explicit column list (deliberately excluding
// embedding/embedding_model/embedded_at, which are frequently NULL and
// aren't representable in pgvector-go's non-nullable Vector type) so their
// generated Row types are structurally identical and freely convertible.
func jobFromRow(row db.GetJobByIDRow) Job {
	return Job{
		ID:                   database.PGToUUID(row.ID),
		Source:               row.Source,
		ExternalID:           row.ExternalID,
		CompanyID:            database.PGToUUID(row.CompanyID),
		CompanyName:          row.CompanyName,
		Title:                row.Title,
		NormalizedTitle:      row.NormalizedTitle,
		Seniority:            database.TextOrNil(row.Seniority),
		Description:          row.Description,
		Country:              database.TextOrNil(row.Country),
		State:                database.TextOrNil(row.State),
		City:                 database.TextOrNil(row.City),
		LocationText:         database.TextOrNil(row.LocationText),
		CountryCode:          database.TextOrNil(row.CountryCode),
		StateCode:            database.TextOrNil(row.StateCode),
		WorkplaceType:        row.WorkplaceType,
		RemoteScope:          row.RemoteScope,
		EligibleCountryCodes: row.EligibleCountryCodes,
		LocationConfidence:   row.LocationConfidence,
		RemoteType:           database.TextOrNil(row.RemoteType),
		EmploymentType:       database.TextOrNil(row.EmploymentType),
		SalaryMin:            database.Int4OrNil(row.SalaryMin),
		SalaryMax:            database.Int4OrNil(row.SalaryMax),
		SalaryCurrency:       database.TextOrNil(row.SalaryCurrency),
		ApplyURL:             database.TextOrNil(row.ApplyUrl),
		SourceURL:            database.TextOrNil(row.SourceUrl),
		PostedAt:             database.TimeOrNil(row.PostedAt),
		FirstSeenAt:          row.FirstSeenAt.Time,
		UpdatedAt:            row.UpdatedAt.Time,
		LastSeenAt:           row.LastSeenAt.Time,
		ContentHash:          row.ContentHash,
		Fingerprint:          row.Fingerprint,
		CanonicalJobID:       database.UUIDPtrOrNil(row.CanonicalJobID),
		Status:               row.Status,
		CreatedAt:            row.CreatedAt.Time,
	}
}

// ListFilter narrows a job listing query. Zero values mean "no filter".
type ListFilter struct {
	Search                   string
	RemoteType               string
	EmploymentType           string
	PostedAfter              *time.Time
	Location                 string // matched against location_text/city/state
	CountryCode              string // exact ISO 3166-1 alpha-2 match
	ExcludeSponsorshipDenied bool   // true for candidates who require visa/transfer support
	RequireRecentH1BHistory   bool   // require recent certified DOL LCA evidence for H-1B candidates
	Sort                     string // "newest" | "salary" | "" (default: first_seen_at desc)
	Limit                    int32
	Offset                   int32
}

// Repository provides access to company/job-source/job records.
type Repository struct {
	q    *db.Queries
	pool *database.Pool
}

// NewRepository builds a Repository from a database pool.
func NewRepository(pool *database.Pool) *Repository {
	return &Repository{q: pool.Queries(), pool: pool}
}

// NewRepositoryFromQueries builds a Repository from an existing sqlc Queries
// value (e.g. bound to a transaction). Primarily for other packages'
// integration tests that need fixture jobs/companies.
func NewRepositoryFromQueries(q *db.Queries) *Repository {
	return &Repository{q: q}
}

// UpsertCompany creates or reuses a company row by normalized name.
func (r *Repository) UpsertCompany(ctx context.Context, name, normalizedName string) (uuid.UUID, error) {
	row, err := r.q.UpsertCompany(ctx, db.UpsertCompanyParams{
		Name:           name,
		NormalizedName: normalizedName,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return database.PGToUUID(row.ID), nil
}

// UpsertJobResult reports whether the upsert inserted a brand new row.
type UpsertJobResult struct {
	Job            Job
	Inserted       bool
	ContentChanged bool
}

// UpsertJob idempotently inserts or updates a canonical job by (source, external_id).
func (r *Repository) UpsertJob(ctx context.Context, in Job) (UpsertJobResult, error) {
	previousHash, previousErr := r.q.GetJobContentHashBySourceExternalID(ctx, db.GetJobContentHashBySourceExternalIDParams{
		Source:     in.Source,
		ExternalID: in.ExternalID,
	})
	existed := true
	if errors.Is(previousErr, pgx.ErrNoRows) {
		existed = false
	} else if previousErr != nil {
		return UpsertJobResult{}, previousErr
	}

	classification := classifyTitle(in.Title)
	eligibleCountryCodes := in.EligibleCountryCodes
	if eligibleCountryCodes == nil {
		eligibleCountryCodes = []string{}
	}
	workplaceType := in.WorkplaceType
	if workplaceType == "" {
		workplaceType = "ONSITE"
	}
	remoteScope := in.RemoteScope
	if remoteScope == "" {
		remoteScope = "UNKNOWN"
	}
	locationConfidence := in.LocationConfidence
	if locationConfidence == "" {
		locationConfidence = "LOW"
	}
	row, err := r.q.UpsertJob(ctx, db.UpsertJobParams{
		Source:                       in.Source,
		ExternalID:                   in.ExternalID,
		CompanyID:                    database.UUIDToPG(in.CompanyID),
		CompanyName:                  in.CompanyName,
		Title:                        in.Title,
		NormalizedTitle:              in.NormalizedTitle,
		Seniority:                    database.PGText(in.Seniority),
		Description:                  in.Description,
		Country:                      database.PGText(in.Country),
		State:                        database.PGText(in.State),
		City:                         database.PGText(in.City),
		LocationText:                 database.PGText(in.LocationText),
		CountryCode:                  database.PGText(in.CountryCode),
		StateCode:                    database.PGText(in.StateCode),
		WorkplaceType:                workplaceType,
		RemoteScope:                  remoteScope,
		EligibleCountryCodes:         eligibleCountryCodes,
		LocationConfidence:           locationConfidence,
		JobFamily:                    classification.Family,
		RoleClassification:           classification.Classification,
		RoleClassificationConfidence: classification.Confidence,
		RemoteType:                   database.PGText(in.RemoteType),
		EmploymentType:               database.PGText(in.EmploymentType),
		SalaryMin:                    database.PGInt4(in.SalaryMin),
		SalaryMax:                    database.PGInt4(in.SalaryMax),
		SalaryCurrency:               database.PGText(in.SalaryCurrency),
		ApplyUrl:                     database.PGText(in.ApplyURL),
		SourceUrl:                    database.PGText(in.SourceURL),
		PostedAt:                     database.PGTimestamptz(in.PostedAt),
		ContentHash:                  in.ContentHash,
		Fingerprint:                  in.Fingerprint,
	})
	if err != nil {
		return UpsertJobResult{}, err
	}
	return UpsertJobResult{
		Job:            jobFromUpsertRow(row),
		Inserted:       row.Inserted,
		ContentChanged: existed && previousHash != in.ContentHash,
	}, nil
}

func jobFromUpsertRow(row db.UpsertJobRow) Job {
	return Job{
		ID:                   database.PGToUUID(row.ID),
		Source:               row.Source,
		ExternalID:           row.ExternalID,
		CompanyID:            database.PGToUUID(row.CompanyID),
		CompanyName:          row.CompanyName,
		Title:                row.Title,
		NormalizedTitle:      row.NormalizedTitle,
		Seniority:            database.TextOrNil(row.Seniority),
		Description:          row.Description,
		Country:              database.TextOrNil(row.Country),
		State:                database.TextOrNil(row.State),
		City:                 database.TextOrNil(row.City),
		LocationText:         database.TextOrNil(row.LocationText),
		CountryCode:          database.TextOrNil(row.CountryCode),
		StateCode:            database.TextOrNil(row.StateCode),
		WorkplaceType:        row.WorkplaceType,
		RemoteScope:          row.RemoteScope,
		EligibleCountryCodes: row.EligibleCountryCodes,
		LocationConfidence:   row.LocationConfidence,
		RemoteType:           database.TextOrNil(row.RemoteType),
		EmploymentType:       database.TextOrNil(row.EmploymentType),
		SalaryMin:            database.Int4OrNil(row.SalaryMin),
		SalaryMax:            database.Int4OrNil(row.SalaryMax),
		SalaryCurrency:       database.TextOrNil(row.SalaryCurrency),
		ApplyURL:             database.TextOrNil(row.ApplyUrl),
		SourceURL:            database.TextOrNil(row.SourceUrl),
		PostedAt:             database.TimeOrNil(row.PostedAt),
		FirstSeenAt:          row.FirstSeenAt.Time,
		UpdatedAt:            row.UpdatedAt.Time,
		LastSeenAt:           row.LastSeenAt.Time,
		ContentHash:          row.ContentHash,
		Fingerprint:          row.Fingerprint,
		CanonicalJobID:       database.UUIDPtrOrNil(row.CanonicalJobID),
		Status:               row.Status,
		CreatedAt:            row.CreatedAt.Time,
	}
}

// GetByID returns a single job.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (Job, error) {
	row, err := r.q.GetJobByID(ctx, database.UUIDToPG(id))
	if err != nil {
		return Job{}, err
	}
	return jobFromRow(row), nil
}

// ErrNoCanonicalMatch is returned by FindCanonicalByFingerprint when no
// other canonical job shares the fingerprint.
var ErrNoCanonicalMatch = errors.New("no canonical job with this fingerprint")

// FindCanonicalByFingerprint looks for an existing, still-canonical job
// (from any source) sharing fingerprint, excluding excludeJobID itself.
// Used for cross-source dedupe: when a new posting's fingerprint matches one
// already ingested from a different source, the new posting is linked to
// that job instead of appearing as a separate result.
func (r *Repository) FindCanonicalByFingerprint(ctx context.Context, fingerprint string, excludeJobID uuid.UUID) (Job, error) {
	// Production repositories use a direct active-only lookup so a newly
	// active posting can never be hidden behind an old CLOSED canonical row.
	// Query-backed test repositories retain the generated-query path below.
	if r.pool != nil {
		var id uuid.UUID
		err := r.pool.QueryRow(ctx, `
			SELECT id
			FROM jobs
			WHERE fingerprint = $1
			  AND fingerprint <> ''
			  AND canonical_job_id IS NULL
			  AND status = 'ACTIVE'
			  AND id <> $2
			ORDER BY first_seen_at ASC
			LIMIT 1
		`, fingerprint, excludeJobID).Scan(&id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Job{}, ErrNoCanonicalMatch
			}
			return Job{}, err
		}
		return r.GetByID(ctx, id)
	}

	row, err := r.q.FindCanonicalByFingerprint(ctx, db.FindCanonicalByFingerprintParams{
		Fingerprint: fingerprint,
		ID:          database.UUIDToPG(excludeJobID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Job{}, ErrNoCanonicalMatch
		}
		return Job{}, err
	}
	return jobFromRow(db.GetJobByIDRow(row)), nil
}

// SetCanonicalJobID marks jobID as a duplicate of canonicalJobID, so listing
// queries (which filter canonical_job_id IS NULL) stop surfacing it as a
// separate result.
func (r *Repository) SetCanonicalJobID(ctx context.Context, jobID, canonicalJobID uuid.UUID) error {
	return r.q.SetCanonicalJobID(ctx, db.SetCanonicalJobIDParams{
		ID:             database.UUIDToPG(jobID),
		CanonicalJobID: database.PGUUID(&canonicalJobID),
	})
}

// PromoteCanonicalJob makes betterJobID the canonical row for a duplicate
// cluster whose previous canonical was oldCanonicalID. Existing duplicate
// pointers are repointed atomically so there is never a chain of canonical
// references.
func (r *Repository) PromoteCanonicalJob(ctx context.Context, betterJobID, oldCanonicalID uuid.UUID) error {
	if r.pool == nil {
		return errors.New("canonical promotion requires a repository backed by a database pool")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		"UPDATE jobs SET canonical_job_id = $1, updated_at = now() WHERE canonical_job_id = $2",
		betterJobID, oldCanonicalID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		"UPDATE jobs SET canonical_job_id = $1, updated_at = now() WHERE id = $2",
		betterJobID, oldCanonicalID,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		"UPDATE jobs SET canonical_job_id = NULL, updated_at = now() WHERE id = $1",
		betterJobID,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) UpdateRoleClassification(ctx context.Context, jobID uuid.UUID, classification RoleClassification) error {
	return r.q.UpdateJobRoleClassification(ctx, db.UpdateJobRoleClassificationParams{
		ID:                           database.UUIDToPG(jobID),
		JobFamily:                    classification.Family,
		RoleClassification:           classification.Classification,
		RoleClassificationConfidence: classification.Confidence,
	})
}

func (r *Repository) UpdateExplicitSponsorshipDenied(ctx context.Context, jobID uuid.UUID, denied bool) error {
	if r.pool == nil {
		return errors.New("sponsorship prefilter update requires a repository backed by a database pool")
	}
	_, err := r.pool.Exec(ctx,
		"UPDATE jobs SET explicit_sponsorship_denied = $2, updated_at = now() WHERE id = $1",
		jobID, denied,
	)
	return err
}

// CloseStaleJobs marks ACTIVE jobs for (source, companyID) CLOSED if they
// weren't touched (last_seen_at) since cutoff, and returns how many were
// closed. Intended to be called once per poll of a source that returns its
// full current listing every time (see CloseStaleJobs SQL doc comment for
// why aggregator sources with a page cap must not use this).
func (r *Repository) CloseStaleJobs(ctx context.Context, source string, companyID uuid.UUID, cutoff time.Time) (int64, error) {
	return r.q.CloseStaleJobs(ctx, db.CloseStaleJobsParams{
		Source:     source,
		CompanyID:  database.UUIDToPG(companyID),
		LastSeenAt: database.PGTimestamptz(&cutoff),
	})
}

// UpdateEmbedding stores a semantic embedding for a job (Phase E).
func (r *Repository) UpdateEmbedding(ctx context.Context, jobID uuid.UUID, vector []float32, model string) error {
	return r.q.UpdateJobEmbedding(ctx, db.UpdateJobEmbeddingParams{
		ID:             database.UUIDToPG(jobID),
		Embedding:      pgvector.NewVector(vector),
		EmbeddingModel: database.PGText(&model),
	})
}

// JobMatch pairs a Job with its cosine distance to a query embedding (lower
// is more similar; 0 = identical, 2 = opposite).
type JobMatch struct {
	Job
	Distance float64
}

// EmbeddingSearchFilter narrows semantic retrieval with the same cheap hard
// filters as ListFilter (remote_type/employment_type/posted_after), applied
// in the same query as the cosine-distance ranking so the ANN index only
// has to rank whatever survives them.
type EmbeddingSearchFilter struct {
	RemoteType               string
	EmploymentType           string
	PostedAfter              *time.Time
	CountryCode              string
	ExcludeSponsorshipDenied bool
	RequireRecentH1BHistory   bool
}

// SearchByEmbedding returns the limit ACTIVE, canonical, already-embedded
// jobs matching filter, closest to vector by cosine distance (Phase G's
// hard-filter + semantic-retrieval stage).
func (r *Repository) SearchByEmbedding(ctx context.Context, vector []float32, limit int32, filter EmbeddingSearchFilter) ([]JobMatch, error) {
	rows, err := r.q.SearchJobsByEmbedding(ctx, db.SearchJobsByEmbeddingParams{
		Embedding: pgvector.NewVector(vector),
		Limit:     limit,
		Column3:   filter.RemoteType,
		Column4:   filter.EmploymentType,
		Column5:   database.PGTimestamptz(filter.PostedAfter),
		Column6:   filter.CountryCode,
		Column7:   filter.ExcludeSponsorshipDenied,
		Column8:   filter.RequireRecentH1BHistory,
	})
	if err != nil {
		return nil, err
	}
	matches := make([]JobMatch, 0, len(rows))
	for _, row := range rows {
		matches = append(matches, JobMatch{
			Job: Job{
				ID:                   database.PGToUUID(row.ID),
				Source:               row.Source,
				ExternalID:           row.ExternalID,
				CompanyID:            database.PGToUUID(row.CompanyID),
				CompanyName:          row.CompanyName,
				Title:                row.Title,
				NormalizedTitle:      row.NormalizedTitle,
				Seniority:            database.TextOrNil(row.Seniority),
				Description:          row.Description,
				Country:              database.TextOrNil(row.Country),
				State:                database.TextOrNil(row.State),
				City:                 database.TextOrNil(row.City),
				LocationText:         database.TextOrNil(row.LocationText),
				CountryCode:          database.TextOrNil(row.CountryCode),
				StateCode:            database.TextOrNil(row.StateCode),
				WorkplaceType:        row.WorkplaceType,
				RemoteScope:          row.RemoteScope,
				EligibleCountryCodes: row.EligibleCountryCodes,
				LocationConfidence:   row.LocationConfidence,
				RemoteType:           database.TextOrNil(row.RemoteType),
				EmploymentType:       database.TextOrNil(row.EmploymentType),
				SalaryMin:            database.Int4OrNil(row.SalaryMin),
				SalaryMax:            database.Int4OrNil(row.SalaryMax),
				SalaryCurrency:       database.TextOrNil(row.SalaryCurrency),
				ApplyURL:             database.TextOrNil(row.ApplyUrl),
				SourceURL:            database.TextOrNil(row.SourceUrl),
				PostedAt:             database.TimeOrNil(row.PostedAt),
				FirstSeenAt:          row.FirstSeenAt.Time,
				UpdatedAt:            row.UpdatedAt.Time,
				LastSeenAt:           row.LastSeenAt.Time,
				ContentHash:          row.ContentHash,
				Fingerprint:          row.Fingerprint,
				CanonicalJobID:       database.UUIDPtrOrNil(row.CanonicalJobID),
				Status:               row.Status,
				CreatedAt:            row.CreatedAt.Time,
			},
			Distance: row.Distance,
		})
	}
	return matches, nil
}

// List returns a page of active jobs matching filter, most-relevant first.
func (r *Repository) List(ctx context.Context, filter ListFilter) ([]Job, int64, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	rows, err := r.q.ListJobs(ctx, db.ListJobsParams{
		Column1:  filter.Search,
		Column2:  filter.RemoteType,
		Column3:  filter.EmploymentType,
		Column4:  database.PGTimestamptz(filter.PostedAfter),
		Column5:  filter.Location,
		Column6:  filter.CountryCode,
		Column7:  filter.Sort,
		Limit:    limit,
		Offset:   filter.Offset,
		Column10: filter.ExcludeSponsorshipDenied,
		Column11: filter.RequireRecentH1BHistory,
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := r.q.CountJobs(ctx, db.CountJobsParams{
		Column1: filter.Search,
		Column2: filter.RemoteType,
		Column3: filter.EmploymentType,
		Column4: database.PGTimestamptz(filter.PostedAfter),
		Column5: filter.Location,
		Column6: filter.CountryCode,
		Column7: filter.ExcludeSponsorshipDenied,
		Column8: filter.RequireRecentH1BHistory,
	})
	if err != nil {
		return nil, 0, err
	}

	jobs := make([]Job, 0, len(rows))
	for _, row := range rows {
		// ListJobsRow and GetJobByIDRow are structurally identical (same
		// explicit column list) so this conversion is a free relabeling, not
		// a runtime cost - see jobFromRow's doc comment.
		jobs = append(jobs, jobFromRow(db.GetJobByIDRow(row)))
	}
	return jobs, total, nil
}

// JobSourceConfig describes a configured board to poll.
type JobSourceConfig struct {
	ID          uuid.UUID
	SourceType  string
	BoardToken  string
	CompanyID   uuid.UUID
	CompanyName string
}

// CreateJobSource inserts a new job source configuration row.
func (r *Repository) CreateJobSource(ctx context.Context, sourceType string, companyID uuid.UUID, boardToken string, enabled bool) (uuid.UUID, error) {
	row, err := r.q.CreateJobSource(ctx, db.CreateJobSourceParams{
		SourceType: sourceType,
		CompanyID:  database.UUIDToPG(companyID),
		BoardToken: boardToken,
		Enabled:    enabled,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return database.PGToUUID(row.ID), nil
}

// SetSourceTypeEnabled toggles all configured shards/connectors for a source
// type. Optional paid providers use this at startup so secrets + an explicit
// environment flag are enough to activate their pre-seeded shards.
// CloseRetiredManualSourceJobs prevents historical company-seeded ATS
// records from remaining ACTIVE forever after scheduled polling is retired.
// A 48-hour grace period avoids immediately hiding a recently-seen posting
// while the market-wide providers populate their canonical replacement.
func (r *Repository) CloseRetiredManualSourceJobs(ctx context.Context) (int64, error) {
	if r.pool == nil {
		return 0, errors.New("retired source cleanup requires a repository backed by a database pool")
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE jobs
		SET status = 'CLOSED', closed_at = now(), updated_at = now()
		WHERE status = 'ACTIVE'
		  AND source IN ('GREENHOUSE', 'LEVER', 'ASHBY', 'SMARTRECRUITERS', 'WORKABLE')
		  AND last_seen_at < now() - INTERVAL '48 hours'
	`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *Repository) SetSourceTypeEnabled(ctx context.Context, sourceType string, enabled bool) error {
	if r.pool == nil {
		return errors.New("source enablement requires a repository backed by a database pool")
	}
	_, err := r.pool.Exec(ctx,
		"UPDATE job_sources SET enabled = $2 WHERE source_type = $1",
		sourceType, enabled,
	)
	return err
}

// ListJobSources returns all enabled job source configurations.
func (r *Repository) ListJobSources(ctx context.Context) ([]JobSourceConfig, error) {
	rows, err := r.q.ListJobSources(ctx)
	if err != nil {
		return nil, err
	}
	configs := make([]JobSourceConfig, 0, len(rows))
	for _, row := range rows {
		configs = append(configs, JobSourceConfig{
			ID:          database.PGToUUID(row.ID),
			SourceType:  row.SourceType,
			BoardToken:  row.BoardToken,
			CompanyID:   database.PGToUUID(row.CompanyID),
			CompanyName: row.CompanyName,
		})
	}
	return configs, nil
}

// ListDueJobSources returns only enabled sources whose configured polling
// interval has elapsed. This keeps authoritative direct ATS boards frequent
// while allowing broad/paid sources to run only a few times per day.
func (r *Repository) ListDueJobSources(ctx context.Context) ([]JobSourceConfig, error) {
	if r.pool == nil {
		return r.ListJobSources(ctx)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT js.id, js.source_type, js.board_token, js.company_id, c.name
		FROM job_sources js
		JOIN companies c ON c.id = js.company_id
		WHERE js.enabled = true
		  AND (
		      js.last_polled_at IS NULL
		      OR js.last_polled_at <= now() - make_interval(mins => js.poll_interval_minutes)
		  )
		ORDER BY js.created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	configs := []JobSourceConfig{}
	for rows.Next() {
		var cfg JobSourceConfig
		if err := rows.Scan(
			&cfg.ID,
			&cfg.SourceType,
			&cfg.BoardToken,
			&cfg.CompanyID,
			&cfg.CompanyName,
		); err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return configs, nil
}

// GetJobSourceByID loads a single job source configuration, regardless of
// its enabled flag (used by the async sync worker, which is dispatched by
// job source ID rather than by iterating the enabled list directly).
func (r *Repository) GetJobSourceByID(ctx context.Context, id uuid.UUID) (JobSourceConfig, error) {
	row, err := r.q.GetJobSourceByID(ctx, database.UUIDToPG(id))
	if err != nil {
		return JobSourceConfig{}, err
	}
	return JobSourceConfig{
		ID:          database.PGToUUID(row.ID),
		SourceType:  row.SourceType,
		BoardToken:  row.BoardToken,
		CompanyID:   database.PGToUUID(row.CompanyID),
		CompanyName: row.CompanyName,
	}, nil
}

// TouchJobSource records the outcome of a poll attempt.
func (r *Repository) TouchJobSource(ctx context.Context, id uuid.UUID, pollErr error) error {
	var errText *string
	if pollErr != nil {
		msg := pollErr.Error()
		errText = &msg
	}
	return r.q.TouchJobSource(ctx, db.TouchJobSourceParams{
		ID:        database.UUIDToPG(id),
		LastError: database.PGText(errText),
	})
}
