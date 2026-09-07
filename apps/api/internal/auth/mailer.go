package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

var ErrMailerNotConfigured = errors.New("auth mailer is not configured")

type Mailer interface {
	Send(ctx context.Context, to, subject, text string) error
}

type resendMailer struct {
	apiKey string
	from   string
	http   *http.Client
}

func NewMailerFromEnv() Mailer {
	apiKey := strings.TrimSpace(os.Getenv("RESEND_API_KEY"))
	from := strings.TrimSpace(os.Getenv("AUTH_EMAIL_FROM"))
	if apiKey != "" && from != "" {
		return &resendMailer{
			apiKey: apiKey,
			from:   from,
			http:   &http.Client{Timeout: 15 * time.Second},
		}
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("AUTH_MAILER_MODE")), "log") {
		return logMailer{}
	}
	return nil
}

func (m *resendMailer) Send(ctx context.Context, to, subject, text string) error {
	body, err := json.Marshal(map[string]any{
		"from":    m.from,
		"to":      []string{to},
		"subject": subject,
		"text":    text,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.http.Do(req)
	if err != nil {
		return fmt.Errorf("send auth email: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("send auth email returned %s", resp.Status)
	}
	return nil
}

type logMailer struct{}

func (logMailer) Send(_ context.Context, to, subject, text string) error {
	slog.Info("development auth email", "to", to, "subject", subject, "body", text)
	return nil
}
