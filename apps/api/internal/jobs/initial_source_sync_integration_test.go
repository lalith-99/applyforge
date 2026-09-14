package jobs

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
)

func TestNewlyEnabledJobSourceEnqueuesImmediateFirstSync(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := database.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("database not reachable: %v", err)
	}

	companyID := uuid.New()
	normalized := fmt.Sprintf("initial-source-sync-%s", companyID.String())
	if _, err := pool.Exec(ctx, `
		INSERT INTO companies (id, name, normalized_name)
		VALUES ($1, 'Initial Source Sync Test', $2)
	`, companyID, normalized); err != nil {
		t.Fatalf("insert company: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM companies WHERE id = $1`, companyID)
	})

	var sourceID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO job_sources (source_type, company_id, board_token, enabled)
		VALUES ('GREENHOUSE', $1, $2, false)
		RETURNING id
	`, companyID, normalized).Scan(&sourceID); err != nil {
		t.Fatalf("insert disabled job source: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `
			DELETE FROM background_jobs
			WHERE job_type = 'sync_job_source'
			  AND payload->>'job_source_id' = $1
		`, sourceID.String())
	})

	assertActiveSyncCount := func(want int) {
		t.Helper()
		var got int
		if err := pool.QueryRow(ctx, `
			SELECT count(*)::int
			FROM background_jobs
			WHERE job_type = 'sync_job_source'
			  AND payload->>'job_source_id' = $1
			  AND status IN ('PENDING', 'RUNNING')
		`, sourceID.String()).Scan(&got); err != nil {
			t.Fatalf("count active sync jobs: %v", err)
		}
		if got != want {
			t.Fatalf("expected %d active first-sync jobs, got %d", want, got)
		}
	}

	assertActiveSyncCount(0)

	if _, err := pool.Exec(ctx, `UPDATE job_sources SET enabled = true WHERE id = $1`, sourceID); err != nil {
		t.Fatalf("enable job source: %v", err)
	}
	assertActiveSyncCount(1)

	// Repeated source promotion/upsert paths may write enabled=true again before
	// the first poll starts. The active-sync uniqueness fence must keep one job.
	if _, err := pool.Exec(ctx, `UPDATE job_sources SET enabled = true WHERE id = $1`, sourceID); err != nil {
		t.Fatalf("re-enable job source: %v", err)
	}
	assertActiveSyncCount(1)
}
