// Package aiusage records AI operation outcomes (latency, status, cache
// hits) so cost/reliability questions ("how many real calls did this
// operation make today", "what's our error rate") can be answered from the
// database instead of scraping logs. See docs/DECISIONS.md "Phase L" and the
// Phase A enrichment cost incident that motivated pulling this forward.
package aiusage

import (
	"context"
	"log/slog"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// Entry is one recorded AI operation outcome.
type Entry struct {
	Operation    string
	Status       string // "SUCCESS" | "ERROR"
	LatencyMS    int64
	CacheHit     bool
	ErrorMessage *string
}

// Repository persists Entry rows to ai_usage.
type Repository struct {
	q *db.Queries
}

// NewRepository builds a Repository from a database pool.
func NewRepository(pool *database.Pool) *Repository {
	return &Repository{q: pool.Queries()}
}

// Record inserts one ai_usage row.
func (r *Repository) Record(ctx context.Context, e Entry) error {
	return r.q.RecordAIUsage(ctx, db.RecordAIUsageParams{
		Operation:    e.Operation,
		Status:       e.Status,
		LatencyMs:    int32(e.LatencyMS),
		CacheHit:     e.CacheHit,
		ErrorMessage: database.PGText(e.ErrorMessage),
	})
}

// RecordAsync fires Record in a goroutine so instrumentation never adds
// latency to (or can fail) the real request path; failures are only logged.
func (r *Repository) RecordAsync(ctx context.Context, e Entry) {
	if r == nil {
		return
	}
	go func() {
		if err := r.Record(context.WithoutCancel(ctx), e); err != nil {
			slog.Error("record ai_usage failed", "operation", e.Operation, "error", err)
		}
	}()
}
