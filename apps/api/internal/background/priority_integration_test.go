package background

import (
	"context"
	"reflect"
	"testing"

	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
)

func TestQueue_ClaimsInteractiveThenSourceSyncThenBulkAI(t *testing.T) {
	q := testdb.OpenTx(t)
	queue := NewQueueFromQueries(q)
	ctx := context.Background()

	// Enqueue bulk AI first to prove FIFO age cannot let enrichment work jump
	// ahead of fresh source acquisition. Interactive user work must still win.
	if err := queue.Enqueue(ctx, "embed_job", map[string]string{"job_id": "job-1"}, 3); err != nil {
		t.Fatalf("enqueue embed job: %v", err)
	}
	if err := queue.Enqueue(ctx, "sync_job_source", map[string]string{"job_source_id": "source-1"}, 3); err != nil {
		t.Fatalf("enqueue source sync: %v", err)
	}
	if err := queue.Enqueue(ctx, "compute_recommendations", map[string]string{"user_id": "user-1"}, 3); err != nil {
		t.Fatalf("enqueue interactive job: %v", err)
	}

	var claimed []string
	worker := NewWorker(queue, "priority-test")
	for _, jobType := range []string{"embed_job", "sync_job_source", "compute_recommendations"} {
		jt := jobType
		worker.Register(jt, func(_ context.Context, _ Job) error {
			claimed = append(claimed, jt)
			return nil
		})
	}

	for range 3 {
		if err := worker.PollOnce(ctx); err != nil {
			t.Fatalf("poll queue: %v", err)
		}
	}

	want := []string{"compute_recommendations", "sync_job_source", "embed_job"}
	if !reflect.DeepEqual(claimed, want) {
		t.Fatalf("claim order = %v, want %v", claimed, want)
	}
}
