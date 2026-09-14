package jobs

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
)

func TestReserveCompanySourceInspectionTargetsPrioritizesSponsorValue(t *testing.T) {
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

	repo := NewRepository(pool)
	type fixture struct {
		id         uuid.UUID
		name       string
		tier       string
		rank       int
		attempts   int
		sourceType string
		confidence float64
		overdue    time.Duration
	}
	fixtures := []fixture{
		{id: uuid.New(), name: "Retrying Hot Inspection", tier: "HOT", rank: 100, attempts: 4, sourceType: "WORKDAY", confidence: 0.95, overdue: time.Hour},
		{id: uuid.New(), name: "Fresh Warm Inspection", tier: "WARM", rank: 700, sourceType: "ICIMS", confidence: 0.95, overdue: time.Hour},
		{id: uuid.New(), name: "Custom Warm Inspection", tier: "WARM", rank: 701, sourceType: "CUSTOM", confidence: 0.95, overdue: time.Hour},
	}

	for _, f := range fixtures {
		normalized := fmt.Sprintf("source-inspection-%s", f.id.String())
		if _, err := pool.Exec(ctx, `
			INSERT INTO companies (id, name, normalized_name)
			VALUES ($1, $2, $3)
		`, f.id, f.name, normalized); err != nil {
			t.Fatalf("insert company %s: %v", f.name, err)
		}
		id := f.id
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), `DELETE FROM companies WHERE id = $1`, id)
		})

		if _, err := pool.Exec(ctx, `
			INSERT INTO company_sponsor_watchlist (
				company_id, employer_normalized_name, watchlist_rank, tier,
				current_fiscal_year, poll_interval_minutes
			) VALUES ($1, $2, $3, $4, 2026, 60)
		`, f.id, normalized, f.rank, f.tier); err != nil {
			t.Fatalf("insert watchlist row %s: %v", f.name, err)
		}

		if _, err := pool.Exec(ctx, `
			INSERT INTO company_source_registry (
				company_id, source_type, board_token, source_url,
				discovery_method, confidence, monitorable,
				inspection_status, inspection_attempt_count, next_inspection_at
			) VALUES ($1, $2, $3, $4, 'CAREER_PAGE', $5, false, 'PENDING', $6, now() - $7::interval)
		`, f.id, f.sourceType, normalized, "https://example.com/"+normalized, f.confidence, f.attempts, f.overdue.String()); err != nil {
			t.Fatalf("insert registry row %s: %v", f.name, err)
		}
	}

	first, err := repo.ReserveCompanySourceInspectionTargets(ctx, 1, time.Hour)
	if err != nil {
		t.Fatalf("reserve first inspection target: %v", err)
	}
	if len(first) != 1 || first[0].CompanyName != "Fresh Warm Inspection" {
		t.Fatalf("expected fresh WARM direct ATS before repeatedly failed HOT candidate, got %+v", first)
	}

	second, err := repo.ReserveCompanySourceInspectionTargets(ctx, 1, time.Hour)
	if err != nil {
		t.Fatalf("reserve second inspection target: %v", err)
	}
	if len(second) != 1 || second[0].CompanyName != "Custom Warm Inspection" {
		t.Fatalf("expected fresh WARM fallback source before repeatedly failed HOT candidate, got %+v", second)
	}

	third, err := repo.ReserveCompanySourceInspectionTargets(ctx, 1, time.Hour)
	if err != nil {
		t.Fatalf("reserve third inspection target: %v", err)
	}
	if len(third) != 1 || third[0].CompanyName != "Retrying Hot Inspection" {
		t.Fatalf("expected repeatedly failed HOT candidate to remain eligible after fresher WARM candidates, got %+v", third)
	}
}
