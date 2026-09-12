package jobs_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
)

func TestICIMSRegistryIdentityCanonicalizedAtDatabaseBoundary(t *testing.T) {
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

	companyID := uuid.New()
	normalizedName := fmt.Sprintf("icims-registry-%s", companyID.String())
	if _, err := tx.Exec(ctx, `
		INSERT INTO companies (id, name, normalized_name)
		VALUES ($1, 'iCIMS Registry Test Company', $2)
	`, companyID, normalizedName); err != nil {
		t.Fatalf("insert company: %v", err)
	}

	urls := []string{
		"https://Jobs-Acme.iCIMS.com/jobs/123/software-engineer/job?mode=job",
		"https://jobs-acme.icims.com/jobs/456/backend-engineer/job",
	}
	for _, sourceURL := range urls {
		if _, err := tx.Exec(ctx, `
			INSERT INTO company_source_registry (
				company_id, source_type, board_token, source_url,
				discovery_method, confidence, monitorable,
				inspection_status, next_inspection_at
			) VALUES (
				$1, 'ICIMS', 'Jobs-Acme.iCIMS.com', $2,
				'JOB_URL', 0.95, false,
				'PENDING', now()
			)
			ON CONFLICT DO NOTHING
		`, companyID, sourceURL); err != nil {
			t.Fatalf("insert iCIMS discovery %q: %v", sourceURL, err)
		}
	}

	var count int
	var boardToken, sourceURL string
	if err := tx.QueryRow(ctx, `
		SELECT count(*)::int, min(board_token), min(source_url)
		FROM company_source_registry
		WHERE company_id = $1
		  AND source_type = 'ICIMS'
	`, companyID).Scan(&count, &boardToken, &sourceURL); err != nil {
		t.Fatalf("load canonical iCIMS registry row: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one tenant-level iCIMS registry row, got %d", count)
	}
	if boardToken != "jobs-acme.icims.com" {
		t.Fatalf("expected lowercase canonical board token, got %q", boardToken)
	}
	if sourceURL != "https://jobs-acme.icims.com" {
		t.Fatalf("expected tenant-root source URL, got %q", sourceURL)
	}
}
