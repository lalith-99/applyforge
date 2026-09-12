package jobs_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/lalithlochan/applyforge/apps/api/internal/background"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobs"
	"github.com/lalithlochan/applyforge/apps/api/internal/testdb"
)

// TestIngestionService_EnqueueSyncTasks_CreatesOneJobPerSource verifies the
// async ingestion path and, critically, that repeated scheduler/admin triggers
// do not accumulate multiple active sync jobs for the same source.
func TestIngestionService_EnqueueSyncTasks_CreatesOneJobPerSource(t *testing.T) {
	q := testdb.OpenTx(t)
	repo := jobs.NewRepositoryFromQueries(q)
	queue := background.NewQueueFromQueries(q)
	svc := jobs.NewIngestionService(repo, queue)
	ctx := context.Background()

	companyID, err := repo.UpsertCompany(ctx, "Acme", fmt.Sprintf("acme-sync-%s", t.Name()))
	if err != nil {
		t.Fatalf("UpsertCompany: %v", err)
	}
	sourceID, err := repo.CreateJobSource(ctx, "GREENHOUSE", companyID, fmt.Sprintf("acme-%s", t.Name()), true)
	if err != nil {
		t.Fatalf("CreateJobSource: %v", err)
	}

	if err := svc.EnqueueSyncTasks(ctx); err != nil {
		t.Fatalf("first EnqueueSyncTasks: %v", err)
	}
	first, err := queue.FindByTypeAndPayload(ctx, jobs.JobTypeSyncSource, jobs.SyncSourcePayload{JobSourceID: sourceID.String()})
	if err != nil {
		t.Fatalf("find first job: %v", err)
	}

	// The source is still due until a worker updates last_polled_at. A second
	// scheduler tick must therefore debounce against the existing active job.
	if err := svc.EnqueueSyncTasks(ctx); err != nil {
		t.Fatalf("second EnqueueSyncTasks: %v", err)
	}
	second, err := queue.FindByTypeAndPayload(ctx, jobs.JobTypeSyncSource, jobs.SyncSourcePayload{JobSourceID: sourceID.String()})
	if err != nil {
		t.Fatalf("find second job: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected one active sync job per source, first=%s second=%s", first.ID, second.ID)
	}

	var payload jobs.SyncSourcePayload
	if err := json.Unmarshal(second.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.JobSourceID != sourceID.String() {
		t.Fatalf("expected job_source_id %s, got %s", sourceID, payload.JobSourceID)
	}
}
