package jobs

import (
	"context"
	"sort"
	"time"
)

// scheduledJobSource carries the small amount of polling metadata needed to
// prioritize a due-source backlog without changing the public JobSourceConfig.
type scheduledJobSource struct {
	config              JobSourceConfig
	sponsorTier         string
	lastPolledAt        *time.Time
	pollIntervalMinutes int
	createdAt           time.Time
}

// ListPrioritizedDueJobSources returns every source that is currently due,
// ordered so high-value direct ATS boards for H-1B sponsor employers are
// enqueued first while sufficiently overdue lower-tier sources can still
// overtake them. This avoids starvation while improving fresh-job coverage
// when the source-sync queue is backlogged.
func (r *Repository) ListPrioritizedDueJobSources(ctx context.Context) ([]JobSourceConfig, error) {
	if r.pool == nil {
		return r.ListDueJobSources(ctx)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT js.id,
		       js.source_type,
		       js.board_token,
		       js.company_id,
		       c.name,
		       COALESCE(w.tier, ''),
		       js.last_polled_at,
		       js.poll_interval_minutes,
		       js.created_at
		FROM job_sources js
		JOIN companies c ON c.id = js.company_id
		LEFT JOIN company_sponsor_watchlist w ON w.company_id = js.company_id
		WHERE js.enabled = true
		  AND (
		      js.last_polled_at IS NULL
		      OR js.last_polled_at <= now() - make_interval(mins => js.poll_interval_minutes)
		  )
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now()
	sources := make([]scheduledJobSource, 0)
	for rows.Next() {
		var source scheduledJobSource
		if err := rows.Scan(
			&source.config.ID,
			&source.config.SourceType,
			&source.config.BoardToken,
			&source.config.CompanyID,
			&source.config.CompanyName,
			&source.sponsorTier,
			&source.lastPolledAt,
			&source.pollIntervalMinutes,
			&source.createdAt,
		); err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.SliceStable(sources, func(i, j int) bool {
		left := sourceScheduleScore(sources[i], now)
		right := sourceScheduleScore(sources[j], now)
		if left == right {
			return sources[i].createdAt.Before(sources[j].createdAt)
		}
		return left > right
	})

	configs := make([]JobSourceConfig, 0, len(sources))
	for _, source := range sources {
		configs = append(configs, source.config)
	}
	return configs, nil
}

func sourceScheduleScore(source scheduledJobSource, now time.Time) float64 {
	intervalMinutes := source.pollIntervalMinutes
	if intervalMinutes <= 0 {
		intervalMinutes = 60
	}

	// A source that has never been polled gets a modest urgency boost. Sponsor
	// and direct-ATS value still decides whether it should run ahead of a HOT
	// board that is already due.
	overdueIntervals := 3.0
	if source.lastPolledAt != nil {
		elapsed := now.Sub(*source.lastPolledAt).Minutes()
		overdueIntervals = elapsed / float64(intervalMinutes)
		if overdueIntervals < 1 {
			overdueIntervals = 1
		}
	}

	return overdueIntervals + sponsorTierScheduleBonus(source.sponsorTier) + sourceTypeScheduleBonus(source.config.SourceType)
}

func sponsorTierScheduleBonus(tier string) float64 {
	switch tier {
	case "HOT":
		return 4
	case "WARM":
		return 3
	case "COOL":
		return 2
	case "COLD":
		return 1
	default:
		return 0
	}
}

func sourceTypeScheduleBonus(sourceType string) float64 {
	switch sourceType {
	case "GREENHOUSE", "LEVER", "ASHBY", "SMARTRECRUITERS", "WORKABLE", "WORKDAY", "ICIMS", "SUCCESSFACTORS":
		return 1.5
	case "CAREER_PAGE":
		return 1
	default:
		return 0
	}
}
