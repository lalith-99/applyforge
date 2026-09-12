package background

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

func openBackgroundQueueTx(t *testing.T) (context.Context, *Queue, interface {
	Exec(context.Context, string, ...any) (pgconnCommandTag, error)
	QueryRow(context.Context, string, ...any) rowScanner
}) {
	t.Helper()
	return nil, nil, nil
}

// These small local interfaces let the test use pgx.Tx without coupling the
// production queue API to test-only raw SQL helpers.
type pgconnCommandTag interface{}
type rowScanner interface {
	Scan(...any) error
}

func TestWorkerPollOnce_ReclaimsExpiredLease(t *testing.T) {
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

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	queue := NewQueueFromQueries(db.New(tx))
	jobID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO background_jobs (
			id, job_type, payload, status, attempts, max_attempts,
			available_at, locked_at, locked_by
		) VALUES ($1, 'lease_recovery_test', '{}', 'RUNNING', 1, 3,
			now(), now() - INTERVAL '3 hours', 'dead-worker')
	`, jobID); err != nil {
		t.Fatalf("insert stale running job: %v", err)
	}

	handled := false
	worker := NewWorker(queue, "replacement-worker")
	worker.Register("lease_recovery_test", func(context.Context, Job) error {
		handled = true
		return nil
	})

	// The data-modifying CTE that requeues expired leases and the claim use one
	// statement snapshot, so PostgreSQL may make the recovered row claimable on
	// this poll or the immediately following poll. Both are correct.
	err = worker.PollOnce(ctx)
	if errors.Is(err, ErrNoJobAvailable) {
		err = worker.PollOnce(ctx)
	}
	if err != nil {
		t.Fatalf("poll recovered job: %v", err)
	}
	if !handled {
		t.Fatal("expected expired RUNNING lease to be reclaimed and handled")
	}

	var status string
	var attempts int
	if err := tx.QueryRow(ctx, `
		SELECT status, attempts
		FROM background_jobs
		WHERE id = $1
	`, jobID).Scan(&status, &attempts); err != nil {
		t.Fatalf("load recovered job: %v", err)
	}
	if status != "COMPLETED" {
		t.Fatalf("expected COMPLETED after recovery, got %s", status)
	}
	if attempts != 2 {
		t.Fatalf("expected reclaimed job to use second attempt, got %d", attempts)
	}
}

func TestWorkerPollOnce_DeadLettersExpiredExhaustedLease(t *testing.T) {
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

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	queue := NewQueueFromQueries(db.New(tx))
	jobID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO background_jobs (
			id, job_type, payload, status, attempts, max_attempts,
			available_at, locked_at, locked_by
		) VALUES ($1, 'lease_exhausted_test', '{}', 'RUNNING', 3, 3,
			now(), now() - INTERVAL '3 hours', 'dead-worker')
	`, jobID); err != nil {
		t.Fatalf("insert exhausted running job: %v", err)
	}

	worker := NewWorker(queue, "replacement-worker")
	if err := worker.PollOnce(ctx); err != nil && !errors.Is(err, ErrNoJobAvailable) {
		t.Fatalf("poll queue: %v", err)
	}

	var status string
	var lockedAt *time.Time
	var lockedBy *string
	var lastError *string
	if err := tx.QueryRow(ctx, `
		SELECT status, locked_at, locked_by, last_error
		FROM background_jobs
		WHERE id = $1
	`, jobID).Scan(&status, &lockedAt, &lockedBy, &lastError); err != nil {
		t.Fatalf("load exhausted job: %v", err)
	}
	if status != "DEAD_LETTER" {
		t.Fatalf("expected DEAD_LETTER, got %s", status)
	}
	if lockedAt != nil || lockedBy != nil {
		t.Fatalf("expected expired lease fields to be cleared, locked_at=%v locked_by=%v", lockedAt, lockedBy)
	}
	if lastError == nil || *lastError != "background job lease expired after 2 hours" {
		t.Fatalf("expected lease-expired diagnostic, got %v", lastError)
	}
}
