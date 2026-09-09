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
