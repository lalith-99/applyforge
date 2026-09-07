package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
)

var ErrSourceHealthUnavailable = errors.New("source health requires a repository backed by a database pool")

type SourcePollOutcome struct {
	JobSourceID uuid.UUID
	SourceType  string
	BoardToken  string
	CompanyName string
	StartedAt   time.Time
	CompletedAt time.Time
	Result      IngestResult
	Err         error
}

type SourceHealth struct {
	JobSourceID uuid.UUID  `json:"job_source_id"`
	SourceType  string     `json:"source_type"`
	BoardToken  string     `json:"board_token"`
	CompanyName string     `json:"company_name"`
	Enabled     bool       `json:"enabled"`
	LastPolledAt *time.Time `json:"last_polled_at,omitempty"`
	LastError   *string    `json:"last_error,omitempty"`

	LastStatus    *string    `json:"last_status,omitempty"`
	LastStartedAt *time.Time `json:"last_started_at,omitempty"`
	LastCompletedAt *time.Time `json:"last_completed_at,omitempty"`
	LastDurationMS *int32     `json:"last_duration_ms,omitempty"`
	LastFetched    *int32     `json:"last_fetched,omitempty"`
	LastInserted   *int32     `json:"last_inserted,omitempty"`
	LastUpdated    *int32     `json:"last_updated,omitempty"`
	LastDeduped    *int32     `json:"last_deduped,omitempty"`
	LastClosed     *int32     `json:"last_closed,omitempty"`
	LastRunError   *string    `json:"last_run_error,omitempty"`
}

type CatalogHealth struct {
	RawDiscovered24H              int64 `json:"raw_discovered_24h"`
	CanonicalUSSoftwarePosted24H  int64 `json:"canonical_us_software_posted_24h"`
	ActiveCanonicalUSSoftware     int64 `json:"active_canonical_us_software"`
	FreshWithDescription24H       int64 `json:"fresh_with_description_24h"`
	FreshWithApplyURL24H          int64 `json:"fresh_with_apply_url_24h"`
}

func (r *Repository) RecordSourcePoll(ctx context.Context, outcome SourcePollOutcome) error {
	if r.pool == nil {
		return ErrSourceHealthUnavailable
	}

	status := "SUCCESS"
	var errText *string
	if outcome.Err != nil {
		status = "ERROR"
		msg := outcome.Err.Error()
		errText = &msg
	}
	completed := outcome.CompletedAt
	if completed.IsZero() {
		completed = time.Now().UTC()
	}
	durationMS := completed.Sub(outcome.StartedAt).Milliseconds()
	if durationMS < 0 {
		durationMS = 0
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO job_source_poll_runs (
			job_source_id, source_type, board_token, company_name,
			started_at, completed_at, duration_ms, status,
			fetched, inserted, updated, deduped, closed, error_message
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14
		)
	`,
		outcome.JobSourceID,
		outcome.SourceType,
		outcome.BoardToken,
		outcome.CompanyName,
		outcome.StartedAt,
		completed,
		durationMS,
		status,
		outcome.Result.Fetched,
		outcome.Result.Inserted,
		outcome.Result.Updated,
		outcome.Result.Deduped,
		outcome.Result.Closed,
		errText,
	)
	return err
}

func (r *Repository) ListSourceHealth(ctx context.Context, limit int) ([]SourceHealth, error) {
	if r.pool == nil {
		return nil, ErrSourceHealthUnavailable
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			js.id,
			js.source_type,
			js.board_token,
			c.name,
			js.enabled,
			js.last_polled_at,
			js.last_error,
			run.status,
			run.started_at,
			run.completed_at,
			run.duration_ms,
			run.fetched,
			run.inserted,
			run.updated,
			run.deduped,
			run.closed,
			run.error_message
		FROM job_sources js
		JOIN companies c ON c.id = js.company_id
		LEFT JOIN LATERAL (
			SELECT status, started_at, completed_at, duration_ms,
				fetched, inserted, updated, deduped, closed, error_message
			FROM job_source_poll_runs
			WHERE job_source_id = js.id
			ORDER BY started_at DESC
			LIMIT 1
		) run ON true
		ORDER BY COALESCE(js.last_polled_at, js.created_at) DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SourceHealth, 0)
	for rows.Next() {
		var (
			id pgtype.UUID
			lastPolled pgtype.Timestamptz
			lastError pgtype.Text
			lastStatus pgtype.Text
			lastStarted pgtype.Timestamptz
			lastCompleted pgtype.Timestamptz
			lastDuration pgtype.Int4
			lastFetched pgtype.Int4
			lastInserted pgtype.Int4
			lastUpdated pgtype.Int4
			lastDeduped pgtype.Int4
			lastClosed pgtype.Int4
			lastRunError pgtype.Text
			item SourceHealth
		)
		if err := rows.Scan(
			&id,
			&item.SourceType,
			&item.BoardToken,
			&item.CompanyName,
			&item.Enabled,
			&lastPolled,
			&lastError,
			&lastStatus,
			&lastStarted,
			&lastCompleted,
			&lastDuration,
			&lastFetched,
			&lastInserted,
			&lastUpdated,
			&lastDeduped,
			&lastClosed,
			&lastRunError,
		); err != nil {
			return nil, err
		}
		item.JobSourceID = database.PGToUUID(id)
		item.LastPolledAt = database.TimeOrNil(lastPolled)
		item.LastError = database.TextOrNil(lastError)
		item.LastStatus = database.TextOrNil(lastStatus)
		item.LastStartedAt = database.TimeOrNil(lastStarted)
		item.LastCompletedAt = database.TimeOrNil(lastCompleted)
		item.LastDurationMS = database.Int4OrNil(lastDuration)
		item.LastFetched = database.Int4OrNil(lastFetched)
		item.LastInserted = database.Int4OrNil(lastInserted)
		item.LastUpdated = database.Int4OrNil(lastUpdated)
		item.LastDeduped = database.Int4OrNil(lastDeduped)
		item.LastClosed = database.Int4OrNil(lastClosed)
		item.LastRunError = database.TextOrNil(lastRunError)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) GetCatalogHealth(ctx context.Context) (CatalogHealth, error) {
	if r.pool == nil {
		return CatalogHealth{}, ErrSourceHealthUnavailable
	}

	var health CatalogHealth
	err := r.pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (
				WHERE first_seen_at >= now() - INTERVAL '24 hours'
			) AS raw_discovered_24h,
			count(*) FILTER (
				WHERE status = 'ACTIVE'
				  AND canonical_job_id IS NULL
				  AND country_code = 'US'
				  AND role_classification = 'IC_SOFTWARE'
				  AND posted_at IS NOT NULL
				  AND posted_at >= now() - INTERVAL '24 hours'
			) AS canonical_us_software_posted_24h,
			count(*) FILTER (
				WHERE status = 'ACTIVE'
				  AND canonical_job_id IS NULL
				  AND country_code = 'US'
				  AND role_classification = 'IC_SOFTWARE'
			) AS active_canonical_us_software,
			count(*) FILTER (
				WHERE status = 'ACTIVE'
				  AND canonical_job_id IS NULL
				  AND country_code = 'US'
				  AND role_classification = 'IC_SOFTWARE'
				  AND posted_at IS NOT NULL
				  AND posted_at >= now() - INTERVAL '24 hours'
				  AND length(trim(description)) > 0
			) AS fresh_with_description_24h,
			count(*) FILTER (
				WHERE status = 'ACTIVE'
				  AND canonical_job_id IS NULL
				  AND country_code = 'US'
				  AND role_classification = 'IC_SOFTWARE'
				  AND posted_at IS NOT NULL
				  AND posted_at >= now() - INTERVAL '24 hours'
				  AND apply_url IS NOT NULL
			) AS fresh_with_apply_url_24h
		FROM jobs
	`).Scan(
		&health.RawDiscovered24H,
		&health.CanonicalUSSoftwarePosted24H,
		&health.ActiveCanonicalUSSoftware,
		&health.FreshWithDescription24H,
		&health.FreshWithApplyURL24H,
	)
	return health, err
}
