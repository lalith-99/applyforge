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

func TestReserveCompanySourceDiscoveryTargetsPrioritizesUncoveredSponsors(t *testing.T) {
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
		id       uuid.UUID
		name     string
		tier     string
		rank     int
		attempts int
		covered  bool
	}
	fixtures := []fixture{
		{id: uuid.New(), name: "Covered Hot", tier: "HOT", rank: 1, covered: true},
		{id: uuid.New(), name: "Retrying Hot", tier: "HOT", rank: 100, attempts: 4},
		{id: uuid.New(), name: "Fresh Warm", tier: "WARM", rank: 700},
		{id: uuid.New(), name: "Fresh Cool", tier: "COOL", rank: 3000},
	}

	for _, f := range fixtures {
		normalized := fmt.Sprintf("source-discovery-%s", f.id.String())
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
				company_id,
				employer_normalized_name,
				watchlist_rank,
				tier,
				current_fiscal_year,
				poll_interval_minutes,
				source_discovery_status,
				source_discovery_attempt_count,
				next_source_discovery_at
			) VALUES ($1, $2, $3, $4, 2026, 60, 'PENDING', $5, now() - interval '1 hour')
		`, f.id, normalized, f.rank, f.tier, f.attempts); err != nil {
			t.Fatalf("insert watchlist row %s: %v", f.name, err)
		}

		if f.covered {
			if _, err := pool.Exec(ctx, `
				INSERT INTO company_source_registry (
					company_id, source_type, board_token, source_url,
					discovery_method, confidence, monitorable
				) VALUES ($1, 'GREENHOUSE', $2, $3, 'CAREER_PAGE', 0.99, true)
			`, f.id, normalized, "https://boards.greenhouse.io/"+normalized); err != nil {
				t.Fatalf("insert monitorable source %s: %v", f.name, err)
			}
		}
	}

	first, err := repo.ReserveCompanySourceDiscoveryTargets(ctx, 1, 30*time.Minute)
	if err != nil {
		t.Fatalf("reserve first target: %v", err)
	}
	if len(first) != 1 || first[0].CompanyName != "Fresh Warm" {
		t.Fatalf("expected fresh WARM sponsor before repeatedly failed HOT source, got %+v", first)
	}

	second, err := repo.ReserveCompanySourceDiscoveryTargets(ctx, 1, 30*time.Minute)
	if err != nil {
		t.Fatalf("reserve second target: %v", err)
	}
	if len(second) != 1 || second[0].CompanyName != "Retrying Hot" {
		t.Fatalf("expected uncovered HOT retry before COOL source, got %+v", second)
	}

	var coveredNext time.Time
	if err := pool.QueryRow(ctx, `
		SELECT next_source_discovery_at
		FROM company_sponsor_watchlist
		WHERE company_id = $1
	`, fixtures[0].id).Scan(&coveredNext); err != nil {
		t.Fatalf("load covered sponsor schedule: %v", err)
	}
	if coveredNext.After(time.Now()) {
		t.Fatalf("covered sponsor should not be reserved for paid rediscovery; next=%s", coveredNext)
	}
}
