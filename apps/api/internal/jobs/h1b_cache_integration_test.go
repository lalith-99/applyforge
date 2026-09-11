package jobs

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCachedH1BHistoryPreservesBrandToLegalEmployerMatch(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
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

	suffix := uuid.NewString()[:8]
	brandName := fmt.Sprintf("h1b cache brand %s", suffix)
	legalEmployerName := brandName + " technologies"

	var companyID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO companies (name, normalized_name)
		VALUES ($1, $1)
		RETURNING id
	`, brandName).Scan(&companyID); err != nil {
		t.Fatalf("insert brand company: %v", err)
	}

	currentFY := time.Now().UTC().Year()
	if time.Now().UTC().Month() >= time.October {
		currentFY++
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO company_immigration_evidence (
			employer_name,
			employer_normalized_name,
			program,
			fiscal_year,
			certified_count,
			denied_count,
			withdrawn_count,
			other_count,
			total_count,
			source_release,
			source_url
		) VALUES ($1, $1, 'LCA_H1B', $2, 3, 0, 0, 0, 3, $3, 'https://example.test/lca')
	`, legalEmployerName, currentFY, "test-"+suffix); err != nil {
		t.Fatalf("insert H-1B evidence: %v", err)
	}

	var uncachedEligible bool
	if err := tx.QueryRow(ctx,
		`SELECT company_has_recent_h1b_history_uncached($1)`, companyID,
	).Scan(&uncachedEligible); err != nil {
		t.Fatalf("evaluate uncached H-1B history: %v", err)
	}
	if !uncachedEligible {
		t.Fatal("expected brand company to match longer legal employer evidence")
	}

	if _, err := tx.Exec(ctx, `SELECT refresh_company_recent_h1b_status()`); err != nil {
		t.Fatalf("refresh cached H-1B status: %v", err)
	}

	var cachedEligible bool
	if err := tx.QueryRow(ctx,
		`SELECT company_has_recent_h1b_history($1)`, companyID,
	).Scan(&cachedEligible); err != nil {
		t.Fatalf("evaluate cached H-1B history: %v", err)
	}
	if !cachedEligible {
		t.Fatal("cached H-1B status lost brand-to-legal employer eligibility")
	}

	var watchlisted bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM company_sponsor_watchlist WHERE company_id = $1
		)
	`, companyID).Scan(&watchlisted); err != nil {
		t.Fatalf("check sponsor watchlist membership: %v", err)
	}
	if watchlisted {
		t.Fatal("test fixture should prove eligibility independently of watchlist membership")
	}
}
