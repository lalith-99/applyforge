package background

import (
	"context"
	"testing"
	"time"

	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
)

func TestQueueEnqueueDebounced_DoesNotDuplicateRecentEquivalentJob(t *testing.T) {
	q := testdb.OpenTx(t)
	queue := NewQueueFromQueries(q)
	ctx := context.Background()
	payload := map[string]string{"user_id": "candidate-1"}

	if err := queue.EnqueueDebounced(ctx, "compute_recommendations", payload, 3, 2*time.Minute); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}
	first, err := queue.FindByTypeAndPayload(ctx, "compute_recommendations", payload)
	if err != nil {
		t.Fatalf("find first job: %v", err)
	}

	if err := queue.EnqueueDebounced(ctx, "compute_recommendations", payload, 3, 2*time.Minute); err != nil {
		t.Fatalf("second enqueue: %v", err)
	}
	second, err := queue.FindByTypeAndPayload(ctx, "compute_recommendations", payload)
	if err != nil {
		t.Fatalf("find second job: %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("expected debounce to keep one recent job, first=%s second=%s", first.ID, second.ID)
	}
}
