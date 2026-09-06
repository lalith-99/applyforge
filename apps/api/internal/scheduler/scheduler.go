// Package scheduler periodically enqueues job-source sync tasks (see
// MASTER_REQUIREMENTS.md §49). It intentionally does nothing beyond
// triggering enqueueing on an interval — actual fetching happens in
// background workers (internal/jobs.SyncSourceWorker), so a slow or
// rate-limited provider never blocks polling the others.
package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// Enqueuer schedules one sync task per configured job source.
type Enqueuer interface {
	EnqueueSyncTasks(ctx context.Context) error
}

// Run enqueues sync tasks via enqueuer.EnqueueSyncTasks every interval until
// ctx is cancelled. It runs an initial enqueue immediately rather than
// waiting a full interval first.
func Run(ctx context.Context, enqueuer Enqueuer, interval time.Duration) {
	if err := enqueuer.EnqueueSyncTasks(ctx); err != nil {
		slog.Error("scheduler: initial job sync enqueue failed", "error", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := enqueuer.EnqueueSyncTasks(ctx); err != nil {
				slog.Error("scheduler: job sync enqueue failed", "error", err)
			}
		}
	}
}
