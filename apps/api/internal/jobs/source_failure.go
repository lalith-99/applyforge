package jobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const permanentSourceErrorPrefix = "PERMANENT_SOURCE: "

// isPermanentSourcePollFailure classifies failures that indicate the configured
// board/account itself is no longer valid. Transient transport/provider errors
// must remain retryable.
func isPermanentSourcePollFailure(sourceType string, err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "returned 404 not found") ||
		strings.Contains(message, "returned 410 gone") {
		return true
	}

	// 422 from the public ATS endpoints below is normally a malformed or retired
	// tenant/site identifier, not a transient provider outage. Do not generalize
	// this to broad providers such as Bright Data/Google Jobs.
	switch strings.ToUpper(strings.TrimSpace(sourceType)) {
	case "GREENHOUSE", "LEVER", "ASHBY", "SMARTRECRUITERS", "WORKABLE", "WORKDAY", "CAREER_PAGE":
		return strings.Contains(message, "returned 422 unprocessable entity")
	default:
		return false
	}
}

func isPermanentSourceErrorText(message string) bool {
	return strings.HasPrefix(strings.TrimSpace(message), permanentSourceErrorPrefix)
}

// QuarantineJobSource disables a permanently invalid direct source and marks
// the matching registry candidate failed. It intentionally leaves the source
// row in place so operators retain history and later rediscovery can repair it
// after the normal discovery refresh window.
func (r *Repository) QuarantineJobSource(ctx context.Context, cfg JobSourceConfig, pollErr error) error {
	if r.pool == nil {
		return ErrSourceHealthUnavailable
	}
	if pollErr == nil {
		return errors.New("cannot quarantine job source without a poll error")
	}

	reason := permanentSourceErrorPrefix + pollErr.Error()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		UPDATE job_sources
		SET enabled = false,
		    last_polled_at = now(),
		    last_error = $2
		WHERE id = $1
	`, cfg.ID, reason); err != nil {
		return fmt.Errorf("disable invalid job source: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE company_source_registry
		SET monitorable = false,
		    inspection_status = 'FAILED',
		    last_error = $4,
		    inspection_last_error = $4,
		    last_inspection_at = now(),
		    next_inspection_at = now() + make_interval(secs => $5),
		    last_seen_at = now()
		WHERE company_id = $1
		  AND source_type = $2
		  AND board_token = $3
	`, cfg.CompanyID, cfg.SourceType, cfg.BoardToken, reason, int((7*24*time.Hour).Seconds())); err != nil {
		return fmt.Errorf("mark invalid source registry candidate: %w", err)
	}

	// Reflect the best remaining discovery state for this sponsor. A different
	// verified source keeps the company RESOLVED; another viable registry
	// candidate keeps it PARTIAL; otherwise this source-discovery path is FAILED.
	if _, err := tx.Exec(ctx, `
		UPDATE company_sponsor_watchlist w
		SET source_discovery_status = CASE
		        WHEN EXISTS (
		            SELECT 1
		            FROM company_source_registry csr
		            WHERE csr.company_id = w.company_id
		              AND csr.monitorable = true
		              AND COALESCE(csr.last_error, '') NOT LIKE $2
		        ) THEN 'RESOLVED'
		        WHEN EXISTS (
		            SELECT 1
		            FROM company_source_registry csr
		            WHERE csr.company_id = w.company_id
		              AND COALESCE(csr.last_error, '') NOT LIKE $2
		        ) THEN 'PARTIAL'
		        ELSE 'FAILED'
		    END,
		    last_source_discovery_at = now(),
		    updated_at = now()
		WHERE w.company_id = $1
	`, cfg.CompanyID, permanentSourceErrorPrefix+"%"); err != nil {
		return fmt.Errorf("update sponsor discovery state after quarantine: %w", err)
	}

	return tx.Commit(ctx)
}
