package jobs

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var ErrProviderBudgetExhausted = errors.New("provider request budget exhausted")

type ProviderRequestBudget struct {
	MaxPerDay        int
	MaxPerMonth      int
	EstimatedCostUSD float64
}

func (b ProviderRequestBudget) validate() error {
	if b.MaxPerDay < 1 {
		return errors.New("provider daily request budget must be positive")
	}
	if b.MaxPerMonth < 1 {
		return errors.New("provider monthly request budget must be positive")
	}
	if b.MaxPerDay > b.MaxPerMonth {
		return errors.New("provider daily request budget cannot exceed monthly request budget")
	}
	if b.EstimatedCostUSD < 0 {
		return errors.New("provider estimated request cost cannot be negative")
	}
	return nil
}

// ReserveProviderUsage atomically checks daily/monthly request caps and
// reserves one paid request before the external provider is called.
// Advisory locking serializes budget reservations across API replicas.
func (r *Repository) ReserveProviderUsage(
	ctx context.Context,
	provider string,
	operation string,
	budget ProviderRequestBudget,
) (uuid.UUID, error) {
	if r.pool == nil {
		return uuid.Nil, errors.New("provider usage reservation requires a database-backed repository")
	}
	if err := budget.validate(); err != nil {
		return uuid.Nil, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	lockKey := provider + ":" + operation
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", lockKey); err != nil {
		return uuid.Nil, fmt.Errorf("lock provider budget: %w", err)
	}

	var dailyCount, monthlyCount int
	if err := tx.QueryRow(ctx, `
		SELECT
			count(*) FILTER (
				WHERE created_at >= (
					date_trunc('day', now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'
				)
			)::int,
			count(*) FILTER (
				WHERE created_at >= (
					date_trunc('month', now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'
				)
			)::int
		FROM provider_usage
		WHERE provider = $1
		  AND operation = $2
		  AND status <> 'CANCELLED'
	`, provider, operation).Scan(&dailyCount, &monthlyCount); err != nil {
		return uuid.Nil, fmt.Errorf("count provider usage: %w", err)
	}

	if dailyCount >= budget.MaxPerDay || monthlyCount >= budget.MaxPerMonth {
		return uuid.Nil, ErrProviderBudgetExhausted
	}

	var usageID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO provider_usage (
			provider,
			operation,
			status,
			units,
			estimated_cost_usd
		)
		VALUES ($1, $2, 'RESERVED', 1, $3)
		RETURNING id
	`, provider, operation, budget.EstimatedCostUSD).Scan(&usageID); err != nil {
		return uuid.Nil, fmt.Errorf("reserve provider usage: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return usageID, nil
}

func (r *Repository) CompleteProviderUsage(
	ctx context.Context,
	usageID uuid.UUID,
	externalRequestID string,
) error {
	if r.pool == nil {
		return errors.New("provider usage completion requires a database-backed repository")
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE provider_usage
		SET status = 'SUCCESS',
		    external_request_id = NULLIF($2, ''),
		    completed_at = now(),
		    error_message = NULL
		WHERE id = $1
	`, usageID, externalRequestID)
	return err
}

func (r *Repository) FailProviderUsage(
	ctx context.Context,
	usageID uuid.UUID,
	externalRequestID string,
	providerErr error,
) error {
	if r.pool == nil {
		return errors.New("provider usage failure requires a database-backed repository")
	}
	message := ""
	if providerErr != nil {
		message = providerErr.Error()
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE provider_usage
		SET status = 'ERROR',
		    external_request_id = NULLIF($2, ''),
		    completed_at = now(),
		    error_message = NULLIF($3, '')
		WHERE id = $1
	`, usageID, externalRequestID, message)
	return err
}
