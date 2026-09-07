package main

import (
	"strings"
	"testing"
)

func TestValidateProductionConfig_RejectsMissingDependencies(t *testing.T) {
	keys := []string{
		"DATABASE_URL", "WEB_BASE_URL", "AI_WORKER_URL", "S3_ENDPOINT",
		"S3_BUCKET", "S3_ACCESS_KEY", "S3_SECRET_KEY", "S3_USE_SSL",
		"REQUIRE_EMAIL_VERIFICATION", "RESEND_API_KEY", "AUTH_EMAIL_FROM",
		"AUTH_MAILER_MODE", "ADMIN_SYNC_TOKEN",
	}
	for _, key := range keys {
		t.Setenv(key, "")
	}
	err := validateProductionConfig("production")
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected missing production config error, got %v", err)
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

	if err := validateProductionConfig("production"); err != nil {
		t.Fatalf("expected valid production config, got %v", err)
	}
}

func TestValidateProductionConfig_DevelopmentAllowsLocalDefaults(t *testing.T) {
	if err := validateProductionConfig("development"); err != nil {
		t.Fatalf("development should allow local defaults: %v", err)
	}
}
