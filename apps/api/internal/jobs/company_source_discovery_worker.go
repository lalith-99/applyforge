package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/background"
)

const JobTypeResolveCompanySource = "resolve_company_source"

type CompanySourceDiscoveryTarget struct {
	CompanyID     uuid.UUID
	CompanyName   string
	WatchlistRank int
	Tier          string
}

type ResolveCompanySourcePayload struct {
	CompanyID   string `json:"company_id"`
	CompanyName string `json:"company_name"`
}

func (r *Repository) ReserveCompanySourceDiscoveryTargets(
	ctx context.Context,
	limit int,
	hold time.Duration,
) ([]CompanySourceDiscoveryTarget, error) {
	if r.pool == nil {
		return nil, errors.New("company source discovery requires a database-backed repository")
	}
	if limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("company source discovery batch size must be between 1 and 1000")
	}
	if hold < time.Minute {
		hold = 30 * time.Minute
	}

	rows, err := r.pool.Query(ctx, `
		WITH candidates AS (
			SELECT w.company_id
			FROM company_sponsor_watchlist w
			WHERE w.source_discovery_status IN ('PENDING', 'PARTIAL', 'FAILED')
			  AND (
			      w.next_source_discovery_at IS NULL
			      OR w.next_source_discovery_at <= now()
			  )
			ORDER BY w.watchlist_rank
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		),
		reserved AS (
			UPDATE company_sponsor_watchlist w
			SET next_source_discovery_at = now() + make_interval(secs => $2)
			FROM candidates c
			WHERE w.company_id = c.company_id
			RETURNING w.company_id, w.watchlist_rank, w.tier
		)
		SELECT reserved.company_id, c.name, reserved.watchlist_rank, reserved.tier
		FROM reserved
		JOIN companies c ON c.id = reserved.company_id
		ORDER BY reserved.watchlist_rank
	`, limit, int(hold.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	targets := make([]CompanySourceDiscoveryTarget, 0, limit)
	for rows.Next() {
		var target CompanySourceDiscoveryTarget
		if err := rows.Scan(
			&target.CompanyID,
			&target.CompanyName,
			&target.WatchlistRank,
			&target.Tier,
		); err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return targets, nil
}

func (r *Repository) MarkCompanySourceDiscoveryOutcome(
	ctx context.Context,
	companyID uuid.UUID,
	status string,
	retryAfter time.Duration,
	discoveryErr error,
	incrementAttempt bool,
) error {
	switch status {
	case "PENDING", "PARTIAL", "RESOLVED", "FAILED":
	default:
		return fmt.Errorf("invalid company source discovery status %q", status)
	}

	var message string
	if discoveryErr != nil {
		message = discoveryErr.Error()
	}
	retrySeconds := int(retryAfter.Seconds())
	if retrySeconds < 0 {
		retrySeconds = 0
	}

	_, err := r.pool.Exec(ctx, `
		UPDATE company_sponsor_watchlist
		SET source_discovery_status = $2,
		    source_discovery_attempt_count = source_discovery_attempt_count
		        + CASE WHEN $3 THEN 1 ELSE 0 END,
		    last_source_discovery_at = CASE WHEN $3 THEN now() ELSE last_source_discovery_at END,
		    next_source_discovery_at = CASE
		        WHEN $2 = 'RESOLVED' THEN NULL
		        ELSE now() + make_interval(secs => $4)
		    END,
		    source_discovery_last_error = NULLIF($5, ''),
		    updated_at = now()
		WHERE company_id = $1
	`, companyID, status, incrementAttempt, retrySeconds, message)
	return err
}

type CompanySourceDiscoveryWorker struct {
	repo     *Repository
	resolver CompanySourceResolver
	budget   ProviderRequestBudget
}

func NewCompanySourceDiscoveryWorker(
	repo *Repository,
	resolver CompanySourceResolver,
	budget ProviderRequestBudget,
) *CompanySourceDiscoveryWorker {
	return &CompanySourceDiscoveryWorker{
		repo:     repo,
		resolver: resolver,
		budget:   budget,
	}
}

func (w *CompanySourceDiscoveryWorker) Handle(ctx context.Context, job background.Job) error {
	var payload ResolveCompanySourcePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode company source discovery payload: %w", err)
	}
	companyID, err := uuid.Parse(payload.CompanyID)
	if err != nil {
		return fmt.Errorf("invalid company id: %w", err)
	}
	if payload.CompanyName == "" {
		return errors.New("company name is required")
	}

	usageID, err := w.repo.ReserveProviderUsage(
		ctx,
		w.resolver.Provider(),
		w.resolver.Operation(),
		w.budget,
	)
	if errors.Is(err, ErrProviderBudgetExhausted) {
		if markErr := w.repo.MarkCompanySourceDiscoveryOutcome(
			ctx,
			companyID,
			"PENDING",
			12*time.Hour,
			nil,
			false,
		); markErr != nil {
			return markErr
		}
		slog.Info("company source discovery deferred by provider budget",
			"company_id", companyID,
			"company_name", payload.CompanyName,
			"provider", w.resolver.Provider(),
		)
		return nil
	}
	if err != nil {
		return fmt.Errorf("reserve source discovery provider usage: %w", err)
	}

	resolution, resolveErr := w.resolver.Resolve(ctx, payload.CompanyName)
	if resolveErr != nil {
		_ = w.repo.FailProviderUsage(ctx, usageID, resolution.ProviderRequestID, resolveErr)
		if markErr := w.repo.MarkCompanySourceDiscoveryOutcome(
			ctx,
			companyID,
			"FAILED",
			24*time.Hour,
			resolveErr,
			true,
		); markErr != nil {
			return markErr
		}
		// Provider failures are retried by the source-discovery scheduler after
		// a long backoff, not by the generic queue seconds later. This prevents
		// accidental repeated paid requests.
		return nil
	}

	if err := w.repo.CompleteProviderUsage(ctx, usageID, resolution.ProviderRequestID); err != nil {
		slog.Error("complete provider usage failed", "usage_id", usageID, "error", err)
	}

	if len(resolution.Sources) == 0 {
		noSourceErr := errors.New("no company career or ATS source found")
		return w.repo.MarkCompanySourceDiscoveryOutcome(
			ctx,
			companyID,
			"FAILED",
			72*time.Hour,
			noSourceErr,
			true,
		)
	}

	if err := w.repo.RecordDiscoveredCompanySources(ctx, companyID, resolution.Sources); err != nil {
		if markErr := w.repo.MarkCompanySourceDiscoveryOutcome(
			ctx,
			companyID,
			"FAILED",
			24*time.Hour,
			err,
			true,
		); markErr != nil {
			return markErr
		}
		return nil
	}

	hasDirectMonitor := false
	for _, source := range resolution.Sources {
		if source.Monitorable {
			hasDirectMonitor = true
			break
		}
	}

	status := "PARTIAL"
	retryAfter := 7 * 24 * time.Hour
	if hasDirectMonitor {
		status = "RESOLVED"
		retryAfter = 0
	}
	return w.repo.MarkCompanySourceDiscoveryOutcome(
		ctx,
		companyID,
		status,
		retryAfter,
		nil,
		true,
	)
}

type CompanySourceDiscoveryScheduler struct {
	repo      *Repository
	queue     *background.Queue
	batchSize int
	hold      time.Duration
}

func NewCompanySourceDiscoveryScheduler(
	repo *Repository,
	queue *background.Queue,
	batchSize int,
	hold time.Duration,
) *CompanySourceDiscoveryScheduler {
	return &CompanySourceDiscoveryScheduler{
		repo:      repo,
		queue:     queue,
		batchSize: batchSize,
		hold:      hold,
	}
}

func (s *CompanySourceDiscoveryScheduler) EnqueueDue(ctx context.Context) (int, error) {
	targets, err := s.repo.ReserveCompanySourceDiscoveryTargets(ctx, s.batchSize, s.hold)
	if err != nil {
		return 0, err
	}

	enqueued := 0
	for _, target := range targets {
		payload := ResolveCompanySourcePayload{
			CompanyID:   target.CompanyID.String(),
			CompanyName: target.CompanyName,
		}
		if err := s.queue.Enqueue(ctx, JobTypeResolveCompanySource, payload, 1); err != nil {
			_ = s.repo.MarkCompanySourceDiscoveryOutcome(
				ctx,
				target.CompanyID,
				"PENDING",
				15*time.Minute,
				err,
				false,
			)
			return enqueued, err
		}
		enqueued++
	}
	return enqueued, nil
}

func (s *CompanySourceDiscoveryScheduler) Run(ctx context.Context, interval time.Duration) {
	if interval < time.Minute {
		interval = 15 * time.Minute
	}

	run := func() {
		count, err := s.EnqueueDue(ctx)
		if err != nil {
			slog.Error("enqueue company source discovery failed", "error", err)
			return
		}
		if count > 0 {
			slog.Info("company source discovery enqueued", "count", count)
		}
	}

	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
