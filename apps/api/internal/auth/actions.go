package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
)

const (
	actionEmailVerify   = "EMAIL_VERIFY"
	actionPasswordReset = "PASSWORD_RESET"
	emailVerifyTTL      = 24 * time.Hour
	passwordResetTTL    = 30 * time.Minute
)

var ErrInvalidActionToken = errors.New("invalid or expired action token")

type ActionService struct {
	pool       *database.Pool
	mailer     Mailer
	webBaseURL string
}

func NewActionService(pool *database.Pool, mailer Mailer, webBaseURL string) *ActionService {
	return &ActionService{
		pool:       pool,
		mailer:     mailer,
		webBaseURL: strings.TrimRight(webBaseURL, "/"),
	}
}

func (s *ActionService) RequestPasswordReset(ctx context.Context, email string) error {
	var (
		userID uuid.UUID
		storedEmail string
	)
	err := s.pool.QueryRow(ctx,
		"SELECT id, email FROM users WHERE lower(email) = lower($1)",
		strings.TrimSpace(email),
	).Scan(&userID, &storedEmail)
	if errors.Is(err, pgx.ErrNoRows) {
		// Deliberately indistinguishable from a real account.
		return nil
	}
	if err != nil {
		return err
	}
	if s.mailer == nil {
		return ErrMailerNotConfigured
	}

	raw, err := s.issue(ctx, userID, actionPasswordReset, passwordResetTTL)
	if err != nil {
		return err
	}
	link := s.webBaseURL + "/reset-password?token=" + raw
	if err := s.mailer.Send(
		ctx,
		storedEmail,
		"Reset your ApplyForge password",
		"Use this one-time link within 30 minutes to reset your ApplyForge password:

"+link+
			"

If you did not request this, you can ignore this email.",
	); err != nil {
		_ = s.invalidateRawToken(context.WithoutCancel(ctx), raw, actionPasswordReset)
		return err
	}
	return nil
}

func (s *ActionService) SendVerification(ctx context.Context, userID uuid.UUID, email string) error {
	if s.mailer == nil {
		return ErrMailerNotConfigured
	}
	raw, err := s.issue(ctx, userID, actionEmailVerify, emailVerifyTTL)
	if err != nil {
		return err
	}
	link := s.webBaseURL + "/verify-email?token=" + raw
	if err := s.mailer.Send(
		ctx,
		email,
		"Verify your ApplyForge email",
		"Verify your email address using this one-time link within 24 hours:

"+link,
	); err != nil {
		_ = s.invalidateRawToken(context.WithoutCancel(ctx), raw, actionEmailVerify)
		return err
	}
	return nil
}

func (s *ActionService) ConfirmEmailVerification(ctx context.Context, rawToken string) error {
	tx, userID, err := s.lockValidToken(ctx, rawToken, actionEmailVerify)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		"UPDATE users SET email_verified_at = COALESCE(email_verified_at, now()), updated_at = now() WHERE id = $1",
		userID,
	); err != nil {
		return err
	}
	if err := markActionTokenUsed(ctx, tx, rawToken, actionEmailVerify); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *ActionService) ConfirmPasswordReset(ctx context.Context, rawToken, newPassword string) error {
	if len(newPassword) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	tx, userID, err := s.lockValidToken(ctx, rawToken, actionPasswordReset)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		"UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1",
		userID, hash,
	); err != nil {
		return err
	}
	if err := markActionTokenUsed(ctx, tx, rawToken, actionPasswordReset); err != nil {
		return err
	}
	// A reset is a credential-compromise boundary: every existing browser/API
	// session is invalidated so the new password is required everywhere.
	if _, err := tx.Exec(ctx, "DELETE FROM sessions WHERE user_id = $1", userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *ActionService) issue(ctx context.Context, userID uuid.UUID, purpose string, ttl time.Duration) (string, error) {
	raw, err := newSessionToken()
	if err != nil {
		return "", err
	}
	hash := hashToken(raw)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		"UPDATE auth_action_tokens SET used_at = now() WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL",
		userID, purpose,
	); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO auth_action_tokens (user_id, purpose, token_hash, expires_at) VALUES ($1,$2,$3,$4)",
		userID, purpose, hash, time.Now().UTC().Add(ttl),
	); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return raw, nil
}

func (s *ActionService) lockValidToken(ctx context.Context, rawToken, purpose string) (pgx.Tx, uuid.UUID, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, uuid.Nil, ErrInvalidActionToken
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, uuid.Nil, err
	}

	var (
		userID uuid.UUID
		expiresAt time.Time
		usedAt *time.Time
	)
	err = tx.QueryRow(ctx,
		"SELECT user_id, expires_at, used_at FROM auth_action_tokens WHERE token_hash = $1 AND purpose = $2 FOR UPDATE",
		hashToken(rawToken), purpose,
	).Scan(&userID, &expiresAt, &usedAt)
	if err != nil {
		_ = tx.Rollback(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, uuid.Nil, ErrInvalidActionToken
		}
		return nil, uuid.Nil, err
	}
	if usedAt != nil || !expiresAt.After(time.Now().UTC()) {
		_ = tx.Rollback(ctx)
		return nil, uuid.Nil, ErrInvalidActionToken
	}
	return tx, userID, nil
}

func markActionTokenUsed(ctx context.Context, tx pgx.Tx, rawToken, purpose string) error {
	tag, err := tx.Exec(ctx,
		"UPDATE auth_action_tokens SET used_at = now() WHERE token_hash = $1 AND purpose = $2 AND used_at IS NULL",
		hashToken(strings.TrimSpace(rawToken)), purpose,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrInvalidActionToken
	}
	return nil
}

func (s *ActionService) invalidateRawToken(ctx context.Context, rawToken, purpose string) error {
	_, err := s.pool.Exec(ctx,
		"UPDATE auth_action_tokens SET used_at = now() WHERE token_hash = $1 AND purpose = $2 AND used_at IS NULL",
		hashToken(rawToken), purpose,
	)
	return err
}

func (s *ActionService) String() string {
	return fmt.Sprintf("ActionService(mailer_configured=%t)", s.mailer != nil)
}
