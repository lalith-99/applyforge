package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

func validateProductionConfig(environment string) error {
	if !strings.EqualFold(strings.TrimSpace(environment), "production") {
		return nil
	}

	required := []string{
		"DATABASE_URL",
		"WEB_BASE_URL",
		"AI_WORKER_URL",
		"S3_ENDPOINT",
		"S3_BUCKET",
		"S3_ACCESS_KEY",
		"S3_SECRET_KEY",
	}
	var missing []string
	for _, key := range required {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("production configuration missing required variables: %s", strings.Join(missing, ", "))
	}

	if err := requireHTTPSURL("WEB_BASE_URL", os.Getenv("WEB_BASE_URL")); err != nil {
		return err
	}
	if err := requireServiceURL("AI_WORKER_URL", os.Getenv("AI_WORKER_URL")); err != nil {
		return err
	}

	if !strings.EqualFold(strings.TrimSpace(os.Getenv("S3_USE_SSL")), "true") {
		return errors.New("production S3_USE_SSL must be true")
	}

	if strings.EqualFold(strings.TrimSpace(os.Getenv("REQUIRE_EMAIL_VERIFICATION")), "true") {
		if strings.TrimSpace(os.Getenv("RESEND_API_KEY")) == "" ||
			strings.TrimSpace(os.Getenv("AUTH_EMAIL_FROM")) == "" {
			return errors.New("email verification is enabled but RESEND_API_KEY/AUTH_EMAIL_FROM are not configured")
		}
		if strings.EqualFold(strings.TrimSpace(os.Getenv("AUTH_MAILER_MODE")), "log") {
			return errors.New("AUTH_MAILER_MODE=log is not allowed in production")
		}
	}

	if strings.TrimSpace(os.Getenv("ADMIN_SYNC_TOKEN")) != "" &&
		len(strings.TrimSpace(os.Getenv("ADMIN_SYNC_TOKEN"))) < 24 {
		return errors.New("production ADMIN_SYNC_TOKEN must be at least 24 characters when configured")
	}

	return nil
}

func requireHTTPSURL(name, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("production %s must be an absolute https URL", name)
	}
	return nil
}

func requireServiceURL(name, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("production %s must be an absolute service URL", name)
	}
	switch parsed.Scheme {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("production %s must use http or https", name)
	}
}
