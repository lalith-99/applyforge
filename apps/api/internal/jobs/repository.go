package jobs

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// Job is the domain representation of a canonical job posting.
type Job struct {
	ID              uuid.UUID
	Source          string
	ExternalID      string
	CompanyID       uuid.UUID
	CompanyName     string
	Title           string
	NormalizedTitle string
	Seniority       *string
	Description     string
	LocationText    *string
	RemoteType      *string
	EmploymentType  *string
	SalaryMin       *int32
	SalaryMax       *int32
	SalaryCurrency  *string
	ApplyURL        *string
	SourceURL       *string
	PostedAt        *time.Time
	FirstSeenAt     time.Time
	UpdatedAt       time.Time
	LastSeenAt      time.Time
	ContentHash     string
	Status          string
	CreatedAt       time.Time
}

func jobFromRow(row db.Job) Job {
	return Job{
		ID:              database.PGToUUID(row.ID),
		Source:          row.Source,
		ExternalID:      row.ExternalID,
		CompanyID:       database.PGToUUID(row.CompanyID),
		CompanyName:     row.CompanyName,
		Title:           row.Title,
		NormalizedTitle: row.NormalizedTitle,
		Seniority:       database.TextOrNil(row.Seniority),
		Description:     row.Description,
		LocationText:    database.TextOrNil(row.LocationText),
		RemoteType:      database.TextOrNil(row.RemoteType),
		EmploymentType:  database.TextOrNil(row.EmploymentType),
		SalaryMin:       database.Int4OrNil(row.SalaryMin),
		SalaryMax:       database.Int4OrNil(row.SalaryMax),
		SalaryCurrency:  database.TextOrNil(row.SalaryCurrency),
		ApplyURL:        database.TextOrNil(row.ApplyUrl),
		SourceURL:       database.TextOrNil(row.SourceUrl),
		PostedAt:        database.TimeOrNil(row.PostedAt),
		FirstSeenAt:     row.FirstSeenAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
		LastSeenAt:      row.LastSeenAt.Time,
		ContentHash:     row.ContentHash,
		Status:          row.Status,
		CreatedAt:       row.CreatedAt.Time,
	}
}

// ListFilter narrows a job listing query. Zero values mean "no filter".
type ListFilter struct {
	Search         string
	RemoteType     string
	EmploymentType string
	PostedAfter    *time.Time
	Location       string // matched against location_text/city/state/country
	Sort           string // "newest" | "salary" | "" (default: first_seen_at desc)
	Limit          int32
	Offset         int32
}

// Repository provides access to company/job-source/job records.
type Repository struct {
	q *db.Queries
}

// NewRepository builds a Repository from a database pool.
func NewRepository(pool *database.Pool) *Repository {
	return &Repository{q: pool.Queries()}
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
	Job      Job
	Inserted bool
}

// UpsertJob idempotently inserts or updates a canonical job by (source, external_id).
func (r *Repository) UpsertJob(ctx context.Context, in Job) (UpsertJobResult, error) {
	row, err := r.q.UpsertJob(ctx, db.UpsertJobParams{
		Source:          in.Source,
		ExternalID:      in.ExternalID,
		CompanyID:       database.UUIDToPG(in.CompanyID),
		CompanyName:     in.CompanyName,
		Title:           in.Title,
		NormalizedTitle: in.NormalizedTitle,
		Seniority:       database.PGText(in.Seniority),
		Description:     in.Description,
		LocationText:    database.PGText(in.LocationText),
		RemoteType:      database.PGText(in.RemoteType),
		EmploymentType:  database.PGText(in.EmploymentType),
		SalaryMin:       database.PGInt4(in.SalaryMin),
		SalaryMax:       database.PGInt4(in.SalaryMax),
		SalaryCurrency:  database.PGText(in.SalaryCurrency),
		ApplyUrl:        database.PGText(in.ApplyURL),
		SourceUrl:       database.PGText(in.SourceURL),
		PostedAt:        database.PGTimestamptz(in.PostedAt),
		ContentHash:     in.ContentHash,
	})
	if err != nil {
		return UpsertJobResult{}, err
	}
	return UpsertJobResult{Job: jobFromUpsertRow(row), Inserted: row.Inserted}, nil
}

func jobFromUpsertRow(row db.UpsertJobRow) Job {
	return Job{
		ID:              database.PGToUUID(row.ID),
		Source:          row.Source,
		ExternalID:      row.ExternalID,
		CompanyID:       database.PGToUUID(row.CompanyID),
		CompanyName:     row.CompanyName,
		Title:           row.Title,
		NormalizedTitle: row.NormalizedTitle,
		Seniority:       database.TextOrNil(row.Seniority),
		Description:     row.Description,
		LocationText:    database.TextOrNil(row.LocationText),
		RemoteType:      database.TextOrNil(row.RemoteType),
		EmploymentType:  database.TextOrNil(row.EmploymentType),
		SalaryMin:       database.Int4OrNil(row.SalaryMin),
		SalaryMax:       database.Int4OrNil(row.SalaryMax),
		SalaryCurrency:  database.TextOrNil(row.SalaryCurrency),
		ApplyURL:        database.TextOrNil(row.ApplyUrl),
		SourceURL:       database.TextOrNil(row.SourceUrl),
		PostedAt:        database.TimeOrNil(row.PostedAt),
		FirstSeenAt:     row.FirstSeenAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
		LastSeenAt:      row.LastSeenAt.Time,
		ContentHash:     row.ContentHash,
		Status:          row.Status,
		CreatedAt:       row.CreatedAt.Time,
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

// List returns a page of active jobs matching filter, most-relevant first.
func (r *Repository) List(ctx context.Context, filter ListFilter) ([]Job, int64, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	rows, err := r.q.ListJobs(ctx, db.ListJobsParams{
		Column1: filter.Search,
		Column2: filter.RemoteType,
		Column3: filter.EmploymentType,
		Column4: database.PGTimestamptz(filter.PostedAfter),
		Column5: filter.Location,
		Column6: filter.Sort,
		Limit:   limit,
		Offset:  filter.Offset,
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
	})
	if err != nil {
		return nil, 0, err
	}

	jobs := make([]Job, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, jobFromRow(row))
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
