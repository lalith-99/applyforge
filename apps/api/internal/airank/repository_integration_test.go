package airank

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobs"
)

func TestRepository_PersistsProviderJudgmentByInputHash(t *testing.T) {
	pool, ctx := openIntegrationPool(t)
	jobsRepo := jobs.NewRepository(pool)
	suffix := uuid.NewString()
	companyID, err := jobsRepo.UpsertCompany(ctx, "Cache Test "+suffix, "cache-test-"+suffix)
	if err != nil {
		t.Fatalf("UpsertCompany: %v", err)
	}
	created, err := jobsRepo.UpsertJob(ctx, jobs.Job{
		Source:          "TEST_CACHE",
		ExternalID:      suffix,
		CompanyID:       companyID,
		CompanyName:     "Cache Test",
		Title:           "Backend Engineer",
		NormalizedTitle: "backend engineer",
		Description:     "Go and Kafka",
	})
	if err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM jobs WHERE id = $1", created.Job.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM companies WHERE id = $1", companyID)
	})

	repo := NewRepository(pool)
	hash := strings.Repeat("a", 64)
	want := Judgment{FitScore: 92, Recommendation: "APPLY_NOW", Reason: "strong fit"}
	if err := repo.SaveJudgments(ctx, []CachedJudgment{{
		InputHash: hash, JobID: created.Job.ID, CacheVersion: "rank-v1:test", Judgment: want,
	}}); err != nil {
		t.Fatalf("SaveJudgments: %v", err)
	}
	loaded, err := repo.LoadJudgments(ctx, []string{hash}, time.Hour)
	if err != nil {
		t.Fatalf("LoadJudgments: %v", err)
	}
	if got, ok := loaded[hash]; !ok || got.FitScore != want.FitScore || got.Recommendation != want.Recommendation {
		t.Fatalf("unexpected cached judgment: %+v", loaded)
	}
}

func TestRepository_RankingBudgetReservationIsHardAndRecoverable(t *testing.T) {
	pool, ctx := openIntegrationPool(t)
	repo := NewRepository(pool)
	cfg := BudgetConfig{
		DailyUSD:            0.01,
		MonthlyUSD:          0.01,
		EstimatedBatchUSD:   0.01,
		ReservationLifetime: time.Minute,
	}
	created := make([]uuid.UUID, 0, 2)
	t.Cleanup(func() {
		for _, id := range created {
			_, _ = pool.Exec(context.Background(), "DELETE FROM ai_budget_debits WHERE id = $1", id)
		}
	})

	first, allowed, err := repo.ReserveRankingBudget(ctx, cfg)
	if err != nil || !allowed {
		t.Fatalf("first reservation: allowed=%v err=%v", allowed, err)
	}
	created = append(created, first.ID)
	if _, allowed, err := repo.ReserveRankingBudget(ctx, cfg); err != nil || allowed {
		t.Fatalf("concurrent reservation must hit hard limit: allowed=%v err=%v", allowed, err)
	}

	if err := repo.FinalizeRankingBudget(ctx, first, false); err != nil {
		t.Fatalf("release reservation: %v", err)
	}
	second, allowed, err := repo.ReserveRankingBudget(ctx, cfg)
	if err != nil || !allowed {
		t.Fatalf("reservation after release: allowed=%v err=%v", allowed, err)
	}
	created = append(created, second.ID)
	if err := repo.FinalizeRankingBudget(ctx, second, true); err != nil {
		t.Fatalf("consume reservation: %v", err)
	}
	if _, allowed, err := repo.ReserveRankingBudget(ctx, cfg); err != nil || allowed {
		t.Fatalf("consumed debit must hold hard limit: allowed=%v err=%v", allowed, err)
	}
}

func openIntegrationPool(t *testing.T) (*database.Pool, context.Context) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	ctx := context.Background()
	pool, err := database.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("database not reachable: %v", err)
	}
	return pool, ctx
}
