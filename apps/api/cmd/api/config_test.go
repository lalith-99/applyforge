package main

import (
	"strings"
	"testing"
	"time"
)

func TestValidateProductionConfig_RejectsMissingDependencies(t *testing.T) {
	keys := []string{
		"DATABASE_URL", "WEB_BASE_URL", "AI_WORKER_URL", "S3_ENDPOINT",
		"S3_BUCKET", "S3_ACCESS_KEY", "S3_SECRET_KEY", "S3_USE_SSL",
		"REQUIRE_EMAIL_VERIFICATION", "RESEND_API_KEY", "AUTH_EMAIL_FROM",
		"AUTH_MAILER_MODE", "ADMIN_SYNC_TOKEN", "BRIGHTDATA_ENABLED",
		"BRIGHTDATA_API_KEY", "BRIGHTDATA_JOBS_DATASET_ID",
	}
	for _, key := range keys {
		t.Setenv(key, "")
	}
	err := validateProductionConfig("production")
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected missing production config error, got %v", err)
	}
}

func TestRankingPolicyFromEnv_DefaultsReserveTailoringBudget(t *testing.T) {
	for _, key := range []string{
		"AI_RANKING_DAILY_BUDGET_USD", "AI_RANKING_MONTHLY_BUDGET_USD",
		"AI_RANKING_ESTIMATED_USD_PER_BATCH", "AI_RANKING_CACHE_TTL_DAYS",
		"AI_RANKING_CACHE_VERSION", "OPENAI_RANKING_MODEL",
	} {
		t.Setenv(key, "")
	}
	cfg, version, ttl, err := rankingPolicyFromEnv()
	if err != nil {
		t.Fatalf("rankingPolicyFromEnv: %v", err)
	}
	if cfg.DailyUSD != 1 || cfg.MonthlyUSD != 10 || cfg.EstimatedBatchUSD != 0.01 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if version != "rank-v1:gpt-5.6-luna" || ttl != 30*24*time.Hour {
		t.Fatalf("unexpected cache policy: version=%q ttl=%s", version, ttl)
	}
}

func TestRankingPolicyFromEnv_RejectsDailyAboveMonthly(t *testing.T) {
	t.Setenv("AI_RANKING_DAILY_BUDGET_USD", "11")
	t.Setenv("AI_RANKING_MONTHLY_BUDGET_USD", "10")
	if _, _, _, err := rankingPolicyFromEnv(); err == nil {
		t.Fatal("expected invalid budget configuration to fail startup")
	}
}

func TestValidateProductionConfig_AcceptsSecureConfiguration(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("WEB_BASE_URL", "https://app.example.com")
	t.Setenv("AI_WORKER_URL", "http://ai-worker.railway.internal:8000")
	t.Setenv("S3_ENDPOINT", "account.r2.cloudflarestorage.com")
	t.Setenv("S3_BUCKET", "applyforge-production")
	t.Setenv("S3_ACCESS_KEY", "access")
	t.Setenv("S3_SECRET_KEY", "secret")
	t.Setenv("S3_USE_SSL", "true")
	t.Setenv("REQUIRE_EMAIL_VERIFICATION", "true")
	t.Setenv("RESEND_API_KEY", "secret")
	t.Setenv("AUTH_EMAIL_FROM", "ApplyForge <noreply@example.com>")
	t.Setenv("AUTH_MAILER_MODE", "")
	t.Setenv("ADMIN_SYNC_TOKEN", strings.Repeat("a", 32))
	t.Setenv("BRIGHTDATA_ENABLED", "true")
	t.Setenv("BRIGHTDATA_API_KEY", "test-market-key")
	t.Setenv("BRIGHTDATA_JOBS_DATASET_ID", "jobs-dataset")

	if err := validateProductionConfig("production"); err != nil {
		t.Fatalf("expected valid production config, got %v", err)
	}
}

func TestValidateProductionConfig_DevelopmentAllowsLocalDefaults(t *testing.T) {
	if err := validateProductionConfig("development"); err != nil {
		t.Fatalf("development should allow local defaults: %v", err)
	}
}

func TestValidateProductionConfig_RejectsProductionWithoutMarketWideJobs(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("WEB_BASE_URL", "https://app.example.com")
	t.Setenv("AI_WORKER_URL", "http://ai-worker.internal:8000")
	t.Setenv("S3_ENDPOINT", "account.r2.cloudflarestorage.com")
	t.Setenv("S3_BUCKET", "applyforge-production")
	t.Setenv("S3_ACCESS_KEY", "access")
	t.Setenv("S3_SECRET_KEY", "secret")
	t.Setenv("S3_USE_SSL", "true")
	t.Setenv("BRIGHTDATA_ENABLED", "false")

	err := validateProductionConfig("production")
	if err == nil || !strings.Contains(err.Error(), "BRIGHTDATA_ENABLED") {
		t.Fatalf("expected market-wide provider requirement, got %v", err)
	}
}
