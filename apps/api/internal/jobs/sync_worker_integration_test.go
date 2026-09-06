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
// new async ingestion path: EnqueueSyncTasks should enqueue a
// sync_job_source background task per enabled job source, rather than
// polling providers synchronously in-process.
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
		t.Fatalf("EnqueueSyncTasks: %v", err)
	}

	found, err := queue.FindByTypeAndPayload(ctx, jobs.JobTypeSyncSource, jobs.SyncSourcePayload{JobSourceID: sourceID.String()})
	if err != nil {
		t.Fatalf("FindByTypeAndPayload: %v", err)
	}

	var payload jobs.SyncSourcePayload
	if err := json.Unmarshal(found.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.JobSourceID != sourceID.String() {
		t.Fatalf("expected job_source_id %s, got %s", sourceID, payload.JobSourceID)
	}
}
