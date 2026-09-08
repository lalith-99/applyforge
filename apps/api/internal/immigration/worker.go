package immigration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/lalithlochan/applyforge/apps/api/internal/background"
)

const JobTypeRefreshSponsorWatchlist = "refresh_sponsor_watchlist"

type RefreshSponsorWatchlistPayload struct {
	Limit int `json:"limit"`
}

type sponsorWatchlistRefresher interface {
	RefreshSponsorWatchlist(ctx context.Context, limit int) (int, error)
}

type SponsorWatchlistRefreshWorker struct {
	refresher sponsorWatchlistRefresher
}

func NewSponsorWatchlistRefreshWorker(refresher sponsorWatchlistRefresher) *SponsorWatchlistRefreshWorker {
	return &SponsorWatchlistRefreshWorker{refresher: refresher}
}

func (w *SponsorWatchlistRefreshWorker) Handle(ctx context.Context, job background.Job) error {
	if w == nil || w.refresher == nil {
		return fmt.Errorf("sponsor watchlist refresher is unavailable")
	}

	var payload RefreshSponsorWatchlistPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode sponsor watchlist refresh payload: %w", err)
	}
	if payload.Limit < 1 || payload.Limit > 50000 {
		return fmt.Errorf("watchlist limit must be between 1 and 50000")
	}

	count, err := w.refresher.RefreshSponsorWatchlist(ctx, payload.Limit)
	if err != nil {
		return fmt.Errorf("refresh sponsor watchlist: %w", err)
	}

	slog.Info(
		"sponsor watchlist refresh completed",
		"job_id", job.ID,
		"requested_limit", payload.Limit,
		"watchlist_companies", count,
	)
	return nil
}
