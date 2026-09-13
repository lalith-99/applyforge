package jobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrSourceLifecycleUnavailable = errors.New("source lifecycle requires a database connection")
	ErrStaleSourcePoll            = errors.New("source poll was superseded by a newer generation")
	ErrSuspiciousEmptySnapshot    = errors.New("refusing to close a non-empty source from an empty snapshot")
)

// sourceLifecycleDB is intentionally small so source-lifecycle integration
// tests can bind a Repository to a rollback-only pgx transaction.
type sourceLifecycleDB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Begin(context.Context) (pgx.Tx, error)
}

// BeginSourcePoll allocates a monotonically increasing fencing token. Any
// older worker that finishes after this call is prevented from publishing its
// snapshot or touching source membership.
func (r *Repository) BeginSourcePoll(ctx context.Context, jobSourceID uuid.UUID) (int64, error) {
	if r.lifecycle == nil {
		return 0, ErrSourceLifecycleUnavailable
	}
	var generation int64
	err := r.lifecycle.QueryRow(ctx, `
		UPDATE job_sources
		SET poll_generation = poll_generation + 1
		WHERE id = $1
		RETURNING poll_generation
	`, jobSourceID).Scan(&generation)
	if err != nil {
		return 0, fmt.Errorf("begin source poll: %w", err)
	}
	return generation, nil
}

// ObserveSourcePosting records the concrete source/tenant that supplied a
// persisted job. The generation predicate fences a stale worker before it can
// revive membership after a newer poll has started.
func (r *Repository) ObserveSourcePosting(
	ctx context.Context,
	jobSourceID uuid.UUID,
	generation int64,
	externalID string,
	jobID uuid.UUID,
	seenAt time.Time,
) error {
	if r.lifecycle == nil {
		return ErrSourceLifecycleUnavailable
	}
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return errors.New("source posting external id is empty")
	}
	tag, err := r.lifecycle.Exec(ctx, `
		INSERT INTO job_source_postings (
			job_source_id, external_id, job_id, first_seen_at, last_seen_at, active
		)
		SELECT $1, $3, $4, $5, $5, true
		FROM job_sources
		WHERE id = $1 AND poll_generation = $2
		ON CONFLICT (job_source_id, external_id) DO UPDATE SET
			job_id = EXCLUDED.job_id,
			last_seen_at = EXCLUDED.last_seen_at,
			active = true
	`, jobSourceID, generation, externalID, jobID, seenAt)
	if err != nil {
		return fmt.Errorf("observe source posting: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrStaleSourcePoll
	}
	return nil
}

// ObserveKnownSourcePostings attaches listing-only IDs (from connectors that
// hydrate just a subset) to already-persisted jobs without inventing new job
// rows. Unknown IDs remain staged only at the provider until hydrated later.
func (r *Repository) ObserveKnownSourcePostings(
	ctx context.Context,
	jobSourceID uuid.UUID,
	generation int64,
	source string,
	companyID uuid.UUID,
	externalIDs []string,
	seenAt time.Time,
) error {
	if r.lifecycle == nil {
		return ErrSourceLifecycleUnavailable
	}
	if len(externalIDs) == 0 {
		return nil
	}
	_, err := r.lifecycle.Exec(ctx, `
		INSERT INTO job_source_postings (
			job_source_id, external_id, job_id, first_seen_at, last_seen_at, active
		)
		SELECT $1, job.external_id, job.id, job.first_seen_at, $6, true
		FROM job_sources source_config
		JOIN jobs job
		  ON job.source = $3
		 AND job.company_id = $4
		 AND job.external_id = ANY($5::text[])
		WHERE source_config.id = $1
		  AND source_config.poll_generation = $2
		ON CONFLICT (job_source_id, external_id) DO UPDATE SET
			job_id = EXCLUDED.job_id,
			last_seen_at = EXCLUDED.last_seen_at,
			active = true
	`, jobSourceID, generation, source, companyID, externalIDs, seenAt)
	if err != nil {
		return fmt.Errorf("observe known source postings: %w", err)
	}
	return nil
}

// FinalizeSourceSnapshot atomically deactivates only memberships absent from
// this concrete source's complete listing. A job closes only when no active
// source membership remains. Empty snapshots are held for investigation when
// the source previously had active inventory.
func (r *Repository) FinalizeSourceSnapshot(
	ctx context.Context,
	jobSourceID uuid.UUID,
	generation int64,
	seenExternalIDs []string,
	pollStartedAt time.Time,
) (int64, error) {
	if r.lifecycle == nil {
		return 0, ErrSourceLifecycleUnavailable
	}

	tx, err := r.lifecycle.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin snapshot finalization: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentGeneration int64
	if err := tx.QueryRow(ctx, `
		SELECT poll_generation
		FROM job_sources
		WHERE id = $1
		FOR UPDATE
	`, jobSourceID).Scan(&currentGeneration); err != nil {
		return 0, fmt.Errorf("lock source snapshot: %w", err)
	}
	if currentGeneration != generation {
		return 0, ErrStaleSourcePoll
	}

	if len(seenExternalIDs) == 0 {
		var activeCount int64
		if err := tx.QueryRow(ctx, `
			SELECT count(*)
			FROM job_source_postings
			WHERE job_source_id = $1 AND active = true
		`, jobSourceID).Scan(&activeCount); err != nil {
			return 0, fmt.Errorf("count source inventory: %w", err)
		}
		if activeCount > 0 {
			return 0, fmt.Errorf("%w: source has %d active postings", ErrSuspiciousEmptySnapshot, activeCount)
		}
	}

	if len(seenExternalIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE job_source_postings
			SET active = true, last_seen_at = $3
			WHERE job_source_id = $1
			  AND external_id = ANY($2::text[])
		`, jobSourceID, seenExternalIDs, pollStartedAt); err != nil {
			return 0, fmt.Errorf("activate seen source postings: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE jobs job
			SET last_seen_at = $3, status = 'ACTIVE', updated_at = now()
			FROM job_source_postings posting
			WHERE posting.job_source_id = $1
			  AND posting.external_id = ANY($2::text[])
			  AND posting.job_id = job.id
		`, jobSourceID, seenExternalIDs, pollStartedAt); err != nil {
			return 0, fmt.Errorf("touch seen jobs: %w", err)
		}
	}

	closed, err := tx.Exec(ctx, `
		WITH deactivated AS (
			UPDATE job_source_postings
			SET active = false
			WHERE job_source_id = $1
			  AND active = true
			  AND last_seen_at < $2
			RETURNING job_id
		)
		UPDATE jobs job
		SET status = 'CLOSED', updated_at = now()
		WHERE job.id IN (SELECT job_id FROM deactivated)
		  AND job.status = 'ACTIVE'
		  AND NOT EXISTS (
			SELECT 1
			FROM job_source_postings remaining
			WHERE remaining.job_id = job.id
			  AND remaining.active = true
		  )
	`, jobSourceID, pollStartedAt)
	if err != nil {
		return 0, fmt.Errorf("close missing source postings: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit snapshot finalization: %w", err)
	}
	return closed.RowsAffected(), nil
}
