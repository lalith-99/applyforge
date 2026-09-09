package jobrecommendations

import (
	"context"
	"log/slog"
	"time"

	"github.com/lalithlochan/applyforge/apps/api/internal/background"
	"github.com/lalithlochan/applyforge/apps/api/internal/candidateprofile"
)

// EnqueueForActiveUsers enqueues a compute_recommendations task for every
// user who has generated a candidate profile - periodic refresh so
// newly-arrived jobs eventually reach existing users even without a
// resume/profile/preferences change to trigger recomputation reactively.
func EnqueueForActiveUsers(ctx context.Context, queue *background.Queue, profiles *candidateprofile.Repository) error {
	userIDs, err := profiles.ListActiveUserIDs(ctx)
	if err != nil {
		return err
	}
	for _, userID := range userIDs {
		if err := queue.EnqueueDebounced(
			ctx,
			JobTypeCompute,
			ComputePayload{UserID: userID.String()},
			3,
			2*time.Minute,
		); err != nil {
			slog.Error("enqueue compute_recommendations failed", "user_id", userID, "error", err)
		}
	}
	return nil
}


// RunCatalogRefreshDebouncer coalesces bursts of catalog-change signals and
// invokes refresh once the catalog has been quiet for quietPeriod. This keeps
// a large source-sync wave from triggering repeated expensive AI reranks.
func RunCatalogRefreshDebouncer(
	ctx context.Context,
	changes <-chan struct{},
	quietPeriod time.Duration,
	refresh func(context.Context) error,
) {
	if quietPeriod <= 0 {
		quietPeriod = 2 * time.Minute
	}

	var timer *time.Timer
	var timerC <-chan time.Time
	resetTimer := func() {
		if timer == nil {
			timer = time.NewTimer(quietPeriod)
			timerC = timer.C
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(quietPeriod)
		timerC = timer.C
	}

	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-changes:
			resetTimer()
		case <-timerC:
			if err := refresh(ctx); err != nil {
				slog.Error("catalog-triggered recommendation refresh failed", "error", err)
			} else {
				slog.Info("catalog-triggered recommendation refresh enqueued")
			}
			timer = nil
			timerC = nil
		}
	}
}
