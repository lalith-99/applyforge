// Package aiusage records AI operation outcomes and economics so reliability
// and cost can be measured from the database instead of inferred from logs.
package aiusage

import (
	"context"
	"log/slog"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
)

type Entry struct {
	Operation        string
	Status           string // SUCCESS | ERROR
	LatencyMS        int64
	CacheHit         bool
	ErrorMessage     *string
	Provider         string
	Model            string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	EstimatedCostUSD *float64
}

type Repository struct {
	pool *database.Pool
}

func NewRepository(pool *database.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Record(ctx context.Context, e Entry) error {
	var provider any
	if e.Provider != "" {
		provider = e.Provider
	}
	var model any
	if e.Model != "" {
		model = e.Model
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO ai_usage (
			operation,
			status,
			latency_ms,
			cache_hit,
			error_message,
			provider,
			model,
			prompt_tokens,
			completion_tokens,
			total_tokens,
			estimated_cost_usd
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`,
		e.Operation,
		e.Status,
		e.LatencyMS,
		e.CacheHit,
		e.ErrorMessage,
		provider,
		model,
		e.PromptTokens,
		e.CompletionTokens,
		e.TotalTokens,
		e.EstimatedCostUSD,
	)
	return err
}

func (r *Repository) RecordAsync(ctx context.Context, e Entry) {
	if r == nil || r.pool == nil {
		return
	}
	go func() {
		if err := r.Record(context.WithoutCancel(ctx), e); err != nil {
			slog.Error("record ai_usage failed", "operation", e.Operation, "error", err)
		}
	}()
}
