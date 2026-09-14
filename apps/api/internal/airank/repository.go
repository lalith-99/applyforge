package airank

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
)

const rankingBudgetLockID int64 = 18367622010793547

// CachedJudgment is one provider-backed judgment keyed by its complete input.
type CachedJudgment struct {
	InputHash    string
	JobID        uuid.UUID
	CacheVersion string
	Judgment     Judgment
}

// BudgetConfig places a conservative hard ceiling on ranking calls. The debit
// is an operator-configured upper estimate per batch, not provider billing.
type BudgetConfig struct {
	DailyUSD            float64
	MonthlyUSD          float64
	EstimatedBatchUSD   float64
	ReservationLifetime time.Duration
}

// BudgetReservation fences one external provider call against concurrent
// workers. A zero ID means no provider call was authorized.
type BudgetReservation struct {
	ID uuid.UUID
}

// PolicyStore provides durable cache and budget state for Service.
type PolicyStore interface {
	LoadJudgments(context.Context, []string, time.Duration) (map[string]Judgment, error)
	SaveJudgments(context.Context, []CachedJudgment) error
	ReserveRankingBudget(context.Context, BudgetConfig) (BudgetReservation, bool, error)
	FinalizeRankingBudget(context.Context, BudgetReservation, bool) error
}

// Repository persists ranking judgments and atomically reserves budget.
type Repository struct {
	pool *database.Pool
}

func NewRepository(pool *database.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) LoadJudgments(ctx context.Context, hashes []string, ttl time.Duration) (map[string]Judgment, error) {
	result := make(map[string]Judgment, len(hashes))
	if len(hashes) == 0 {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT input_hash, judgment
		FROM ai_ranking_judgment_cache
		WHERE input_hash = ANY($1::text[])
		  AND created_at >= now() - make_interval(secs => $2::double precision)
	`, hashes, ttl.Seconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var hash string
		var payload []byte
		if err := rows.Scan(&hash, &payload); err != nil {
			return nil, err
		}
		var judgment Judgment
		if err := json.Unmarshal(payload, &judgment); err != nil {
			return nil, fmt.Errorf("decode cached ranking judgment %s: %w", hash, err)
		}
		result[hash] = judgment
	}
	return result, rows.Err()
}

func (r *Repository) SaveJudgments(ctx context.Context, entries []CachedJudgment) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	for _, entry := range entries {
		payload, err := json.Marshal(entry.Judgment)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO ai_ranking_judgment_cache (
				input_hash, job_id, cache_version, judgment
			) VALUES ($1, $2, $3, $4::jsonb)
			ON CONFLICT (input_hash) DO UPDATE SET
				judgment = EXCLUDED.judgment,
				last_used_at = now()
		`, entry.InputHash, entry.JobID, entry.CacheVersion, payload); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *Repository) ReserveRankingBudget(ctx context.Context, cfg BudgetConfig) (BudgetReservation, bool, error) {
	if cfg.DailyUSD <= 0 || cfg.MonthlyUSD <= 0 || cfg.EstimatedBatchUSD <= 0 {
		return BudgetReservation{}, false, nil
	}
	if cfg.DailyUSD > cfg.MonthlyUSD {
		return BudgetReservation{}, false, errors.New("daily AI ranking budget cannot exceed monthly budget")
	}
	if cfg.ReservationLifetime <= 0 {
		cfg.ReservationLifetime = 10 * time.Minute
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return BudgetReservation{}, false, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", rankingBudgetLockID); err != nil {
		return BudgetReservation{}, false, err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM ai_budget_debits
		WHERE operation = 'rank_jobs'
		  AND status = 'RESERVED'
		  AND expires_at <= now()
	`); err != nil {
		return BudgetReservation{}, false, err
	}

	var daily, monthly float64
	if err := tx.QueryRow(ctx, `
		SELECT
			COALESCE(sum(estimated_cost_usd) FILTER (
				WHERE created_at >= date_trunc('day', now())
			), 0)::double precision,
			COALESCE(sum(estimated_cost_usd) FILTER (
				WHERE created_at >= date_trunc('month', now())
			), 0)::double precision
		FROM ai_budget_debits
		WHERE operation = 'rank_jobs'
		  AND (status = 'CONSUMED' OR expires_at > now())
	`).Scan(&daily, &monthly); err != nil {
		return BudgetReservation{}, false, err
	}
	if daily+cfg.EstimatedBatchUSD > cfg.DailyUSD || monthly+cfg.EstimatedBatchUSD > cfg.MonthlyUSD {
		if err := tx.Commit(ctx); err != nil {
			return BudgetReservation{}, false, err
		}
		return BudgetReservation{}, false, nil
	}

	id := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO ai_budget_debits (
			id, operation, status, estimated_cost_usd, expires_at
		) VALUES ($1, 'rank_jobs', 'RESERVED', $2,
			now() + make_interval(secs => $3::double precision))
	`, id, cfg.EstimatedBatchUSD, cfg.ReservationLifetime.Seconds()); err != nil {
		return BudgetReservation{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return BudgetReservation{}, false, err
	}
	return BudgetReservation{ID: id}, true, nil
}

func (r *Repository) FinalizeRankingBudget(ctx context.Context, reservation BudgetReservation, consumed bool) error {
	if reservation.ID == uuid.Nil {
		return nil
	}
	if !consumed {
		_, err := r.pool.Exec(ctx, "DELETE FROM ai_budget_debits WHERE id = $1 AND status = 'RESERVED'", reservation.ID)
		return err
	}
	command, err := r.pool.Exec(ctx, `
		UPDATE ai_budget_debits
		SET status = 'CONSUMED', consumed_at = now(), expires_at = now()
		WHERE id = $1 AND status = 'RESERVED'
	`, reservation.ID)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return errors.New("AI ranking budget reservation was not active")
	}
	return nil
}
