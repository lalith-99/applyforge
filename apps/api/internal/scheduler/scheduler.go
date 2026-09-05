// Package scheduler periodically polls configured job sources (see
// MASTER_REQUIREMENTS.md §49). It intentionally does nothing beyond
// triggering ingestion on an interval — no distributed coordination, since a
// single api instance is sufficient for the MVP's scale.
package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// Syncer performs one full poll of all configured job sources.
type Syncer interface {
	SyncAll(ctx context.Context) error
}

// Run polls syncer.SyncAll every interval until ctx is cancelled. It runs an
// initial sync immediately rather than waiting a full interval first.
func Run(ctx context.Context, syncer Syncer, interval time.Duration) {
	if err := syncer.SyncAll(ctx); err != nil {
		slog.Error("scheduler: initial job sync failed", "error", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := syncer.SyncAll(ctx); err != nil {
				slog.Error("scheduler: job sync failed", "error", err)
			}
		}
	}
}
