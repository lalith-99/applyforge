package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/background"
)

const JobTypeInspectCompanySource = "inspect_company_source"

type CompanySourceInspectionTarget struct {
	RegistryID   uuid.UUID
	CompanyID    uuid.UUID
	CompanyName  string
	SourceType   string
	SourceURL    string
	WatchlistRank int
}

type InspectCompanySourcePayload struct {
	RegistryID  string `json:"registry_id"`
	CompanyID   string `json:"company_id"`
	CompanyName string `json:"company_name"`
	SourceType  string `json:"source_type"`
	SourceURL   string `json:"source_url"`
}

func (r *Repository) ReserveCompanySourceInspectionTargets(
	ctx context.Context,
	limit int,
	hold time.Duration,
) ([]CompanySourceInspectionTarget, error) {
	if r.pool == nil {
		return nil, errors.New("company source inspection requires a database-backed repository")
	}
	if limit < 1 || limit > 500 {
		return nil, fmt.Errorf("company source inspection batch size must be between 1 and 500")
	}
	if hold < time.Minute {
		hold = time.Hour
	}

	rows, err := r.pool.Query(ctx, `
		WITH candidates AS (
			SELECT csr.id
			FROM company_source_registry csr
			JOIN company_sponsor_watchlist w ON w.company_id = csr.company_id
			WHERE csr.monitorable = false
			  AND csr.source_type IN ('WORKDAY', 'ICIMS', 'ORACLE', 'CUSTOM')
			  AND csr.inspection_status IN ('PENDING', 'FAILED')
			  AND (
			      csr.next_inspection_at IS NULL
			      OR csr.next_inspection_at <= now()
			  )
			ORDER BY w.watchlist_rank, csr.confidence DESC, csr.first_seen_at
			LIMIT $1
			FOR UPDATE OF csr SKIP LOCKED
		),
		reserved AS (
			UPDATE company_source_registry csr
			SET next_inspection_at = now() + make_interval(secs => $2)
			FROM candidates c
			WHERE csr.id = c.id
			RETURNING csr.id, csr.company_id, csr.source_type, csr.source_url
		)
		SELECT
			reserved.id,
			reserved.company_id,
			c.name,
			reserved.source_type,
			reserved.source_url,
			w.watchlist_rank
		FROM reserved
		JOIN companies c ON c.id = reserved.company_id
		JOIN company_sponsor_watchlist w ON w.company_id = reserved.company_id
		ORDER BY w.watchlist_rank
	`, limit, int(hold.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	targets := make([]CompanySourceInspectionTarget, 0, limit)
	for rows.Next() {
		var target CompanySourceInspectionTarget
		if err := rows.Scan(
			&target.RegistryID,
			&target.CompanyID,
			&target.CompanyName,
			&target.SourceType,
			&target.SourceURL,
			&target.WatchlistRank,
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

func (r *Repository) MarkCompanySourceInspectionOutcome(
	ctx context.Context,
	registryID uuid.UUID,
	status string,
	retryAfter time.Duration,
	inspectionErr error,
) error {
	switch status {
	case "PENDING", "RESOLVED", "FAILED", "UNSUPPORTED":
	default:
		return fmt.Errorf("invalid source inspection status %q", status)
	}

	message := ""
	if inspectionErr != nil {
		message = inspectionErr.Error()
	}
	retrySeconds := int(retryAfter.Seconds())
	if retrySeconds < 0 {
		retrySeconds = 0
	}

	_, err := r.pool.Exec(ctx, `
		UPDATE company_source_registry
		SET inspection_status = $2,
		    inspection_attempt_count = inspection_attempt_count + 1,
		    last_inspection_at = now(),
		    next_inspection_at = CASE
		        WHEN $2 IN ('RESOLVED', 'UNSUPPORTED') THEN NULL
		        ELSE now() + make_interval(secs => $3)
		    END,
		    inspection_last_error = NULLIF($4, ''),
		    last_verified_at = CASE
		        WHEN $2 = 'RESOLVED' THEN now()
		        ELSE last_verified_at
		    END
		WHERE id = $1
	`, registryID, status, retrySeconds, message)
	return err
}

type CompanySourceInspectionWorker struct {
	repo      *Repository
	inspector *CareerPageInspector
}

func NewCompanySourceInspectionWorker(
	repo *Repository,
	inspector *CareerPageInspector,
) *CompanySourceInspectionWorker {
	return &CompanySourceInspectionWorker{
		repo:      repo,
		inspector: inspector,
	}
}

func (w *CompanySourceInspectionWorker) Handle(ctx context.Context, job background.Job) error {
	var payload InspectCompanySourcePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode company source inspection payload: %w", err)
	}
	registryID, err := uuid.Parse(payload.RegistryID)
	if err != nil {
		return fmt.Errorf("invalid source registry id: %w", err)
	}
	companyID, err := uuid.Parse(payload.CompanyID)
	if err != nil {
		return fmt.Errorf("invalid company id: %w", err)
	}
	if strings.TrimSpace(payload.SourceURL) == "" {
		return errors.New("source URL is required")
	}

	inspection, inspectErr := w.inspector.Inspect(ctx, payload.SourceURL)
	if inspectErr != nil {
		if markErr := w.repo.MarkCompanySourceInspectionOutcome(
			ctx,
			registryID,
			"FAILED",
			30*24*time.Hour,
			inspectErr,
		); markErr != nil {
			return markErr
		}
		slog.Info("company source inspection failed",
			"company_id", companyID,
			"company_name", payload.CompanyName,
			"source_type", payload.SourceType,
			"source_url", payload.SourceURL,
			"error", inspectErr,
		)
		return nil
	}

	discoveries := make([]DiscoveredCompanySource, 0, len(inspection.Sources)+1)
	for _, source := range inspection.Sources {
		source.DiscoveryMethod = "CAREER_PAGE"
		discoveries = append(discoveries, source)
	}

	if len(inspection.Jobs) > 0 {
		sourceURL := strings.TrimSpace(inspection.FinalURL)
		if sourceURL == "" {
			sourceURL = strings.TrimSpace(payload.SourceURL)
		}
		discoveries = append(discoveries, DiscoveredCompanySource{
			SourceType:      "CAREER_PAGE",
			BoardToken:      sourceURL,
			SourceURL:       sourceURL,
			DiscoveryMethod: "CAREER_PAGE",
			Confidence:      0.9,
			Monitorable:     true,
		})
	}

	if len(discoveries) == 0 {
		noSignalErr := errors.New("career page exposed no supported ATS links or structured JobPosting data")
		if err := w.repo.MarkCompanySourceInspectionOutcome(
			ctx,
			registryID,
			"FAILED",
			30*24*time.Hour,
			noSignalErr,
		); err != nil {
			return err
		}
		return nil
	}

	if err := w.repo.RecordDiscoveredCompanySources(ctx, companyID, discoveries); err != nil {
		if markErr := w.repo.MarkCompanySourceInspectionOutcome(
			ctx,
			registryID,
			"FAILED",
			7*24*time.Hour,
			err,
		); markErr != nil {
			return markErr
		}
		return nil
	}

	hasMonitor := false
	for _, source := range discoveries {
		if source.Monitorable {
			hasMonitor = true
			break
		}
	}
	if !hasMonitor {
		noMonitorErr := errors.New("career page revealed only registry-only ATS sources")
		if err := w.repo.MarkCompanySourceInspectionOutcome(
			ctx,
			registryID,
			"FAILED",
			30*24*time.Hour,
			noMonitorErr,
		); err != nil {
			return err
		}
		return nil
	}

	return w.repo.MarkCompanySourceInspectionOutcome(
		ctx,
		registryID,
		"RESOLVED",
		0,
		nil,
	)
}

type CompanySourceInspectionScheduler struct {
	repo      *Repository
	queue     *background.Queue
	batchSize int
	hold      time.Duration
}

func NewCompanySourceInspectionScheduler(
	repo *Repository,
	queue *background.Queue,
	batchSize int,
	hold time.Duration,
) *CompanySourceInspectionScheduler {
	return &CompanySourceInspectionScheduler{
		repo:      repo,
		queue:     queue,
		batchSize: batchSize,
		hold:      hold,
	}
}

func (s *CompanySourceInspectionScheduler) EnqueueDue(ctx context.Context) (int, error) {
	targets, err := s.repo.ReserveCompanySourceInspectionTargets(ctx, s.batchSize, s.hold)
	if err != nil {
		return 0, err
	}

	enqueued := 0
	for _, target := range targets {
		payload := InspectCompanySourcePayload{
			RegistryID:  target.RegistryID.String(),
			CompanyID:   target.CompanyID.String(),
			CompanyName: target.CompanyName,
			SourceType:  target.SourceType,
			SourceURL:   target.SourceURL,
		}
		if err := s.queue.Enqueue(ctx, JobTypeInspectCompanySource, payload, 1); err != nil {
			_ = s.repo.MarkCompanySourceInspectionOutcome(
				ctx,
				target.RegistryID,
				"FAILED",
				time.Hour,
				err,
			)
			return enqueued, err
		}
		enqueued++
	}
	return enqueued, nil
}

func (s *CompanySourceInspectionScheduler) Run(ctx context.Context, interval time.Duration) {
	if interval < time.Minute {
		interval = 30 * time.Minute
	}

	run := func() {
		count, err := s.EnqueueDue(ctx)
		if err != nil {
			slog.Error("enqueue company source inspection failed", "error", err)
			return
		}
		if count > 0 {
			slog.Info("company source inspection enqueued", "count", count)
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
