package auth

import (
	"context"
	"errors"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	"github.com/lalithlochan/applyforge/apps/api/internal/users"
)

type captureMailer struct {
	mu      sync.Mutex
	to      string
	subject string
	text    string
}

func (m *captureMailer) Send(_ context.Context, to, subject, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.to = to
	m.subject = subject
	m.text = text
	return nil
}

func (m *captureMailer) tokenFromMessage(t *testing.T) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()

	re := regexp.MustCompile(`token=([A-Za-z0-9_-]+)`)
	match := re.FindStringSubmatch(m.text)
	if len(match) != 2 {
		t.Fatalf("expected one-time token link in mail body: %q", m.text)
	}
	return match[1]
}

func openAuthIntegrationPool(t *testing.T) *database.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping auth integration test")
	}
	ctx := context.Background()
	pool, err := database.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("database not reachable: %v", err)
	}
	return pool
}

func TestActionService_EmailVerificationTokenIsSingleUse(t *testing.T) {
	ctx := context.Background()
	pool := openAuthIntegrationPool(t)
	userRepo := users.NewRepository(pool)
	authSvc := NewService(pool, userRepo, GoogleConfig{})
	mailer := &captureMailer{}
	actions := NewActionService(pool, mailer, "https://applyforge.example")

	email := "verify-" + uuid.NewString() + "@example.com"
	issued, err := authSvc.SignUp(ctx, email, "supersecret1", "integration-test", "127.0.0.1")
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	t.Cleanup(func() { _ = userRepo.Delete(context.Background(), issued.User.ID) })

	if issued.User.EmailVerifiedAt != nil {
		t.Fatal("password signup should begin unverified")
	}
	if err := actions.SendVerification(ctx, issued.User.ID, email); err != nil {
		t.Fatalf("send verification: %v", err)
	}
	token := mailer.tokenFromMessage(t)

	if err := actions.ConfirmEmailVerification(ctx, token); err != nil {
		t.Fatalf("confirm verification: %v", err)
	}
	updated, err := userRepo.GetByID(ctx, issued.User.ID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if updated.EmailVerifiedAt == nil {
		t.Fatal("expected email_verified_at to be set")
	}

	if err := actions.ConfirmEmailVerification(ctx, token); !errors.Is(err, ErrInvalidActionToken) {
		t.Fatalf("verification token must be single-use, got %v", err)
	}
}

func TestActionService_PasswordResetInvalidatesExistingSessions(t *testing.T) {
	ctx := context.Background()
	pool := openAuthIntegrationPool(t)
	userRepo := users.NewRepository(pool)
	authSvc := NewService(pool, userRepo, GoogleConfig{})
	mailer := &captureMailer{}
	actions := NewActionService(pool, mailer, "https://applyforge.example")

	email := "reset-" + uuid.NewString() + "@example.com"
	issued, err := authSvc.SignUp(ctx, email, "old-password-123", "integration-test", "127.0.0.1")
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	t.Cleanup(func() { _ = userRepo.Delete(context.Background(), issued.User.ID) })

	if err := actions.RequestPasswordReset(ctx, strings.ToUpper(email)); err != nil {
		t.Fatalf("request password reset: %v", err)
	}
	token := mailer.tokenFromMessage(t)

	if err := actions.ConfirmPasswordReset(ctx, token, "new-password-456"); err != nil {
		t.Fatalf("confirm password reset: %v", err)
	}

	if _, err := authSvc.CurrentUser(ctx, issued.Token); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("existing session must be invalidated after reset, got %v", err)
	}
	if _, err := authSvc.Login(ctx, email, "old-password-123", "integration-test", "127.0.0.1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password must stop working, got %v", err)
	}
	if _, err := authSvc.Login(ctx, email, "new-password-456", "integration-test", "127.0.0.1"); err != nil {
		t.Fatalf("new password should work: %v", err)
	}
	if err := actions.ConfirmPasswordReset(ctx, token, "another-password-789"); !errors.Is(err, ErrInvalidActionToken) {
		t.Fatalf("reset token must be single-use, got %v", err)
	}
}

func TestActionService_PasswordResetRequestDoesNotRevealUnknownEmail(t *testing.T) {
	ctx := context.Background()
	pool := openAuthIntegrationPool(t)
	mailer := &captureMailer{}
	actions := NewActionService(pool, mailer, "https://applyforge.example")

	if err := actions.RequestPasswordReset(ctx, "unknown-"+uuid.NewString()+"@example.com"); err != nil {
		t.Fatalf("unknown email should be indistinguishable from success: %v", err)
	}
	if mailer.text != "" {
		t.Fatal("unknown account must not produce an email")
	}
}
