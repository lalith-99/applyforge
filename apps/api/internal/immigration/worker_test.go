package immigration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/lalithlochan/applyforge/apps/api/internal/background"
)

type fakeSponsorWatchlistRefresher struct {
	limit int
	count int
	err   error
}

func (f *fakeSponsorWatchlistRefresher) RefreshSponsorWatchlist(
	_ context.Context,
	limit int,
) (int, error) {
	f.limit = limit
	return f.count, f.err
}

func TestSponsorWatchlistRefreshWorker(t *testing.T) {
	refresher := &fakeSponsorWatchlistRefresher{count: 10000}
	worker := NewSponsorWatchlistRefreshWorker(refresher)
	payload, err := json.Marshal(RefreshSponsorWatchlistPayload{Limit: 10000})
	if err != nil {
		t.Fatal(err)
	}

	err = worker.Handle(context.Background(), background.Job{
		ID:      "test-job",
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if refresher.limit != 10000 {
		t.Fatalf("expected limit 10000, got %d", refresher.limit)
	}
}

func TestSponsorWatchlistRefreshWorkerRejectsInvalidPayload(t *testing.T) {
	refresher := &fakeSponsorWatchlistRefresher{}
	worker := NewSponsorWatchlistRefreshWorker(refresher)

	err := worker.Handle(context.Background(), background.Job{
		Payload: []byte(`{"limit":0}`),
	})
	if err == nil {
		t.Fatal("expected invalid limit error")
	}
	if refresher.limit != 0 {
		t.Fatalf("refresher should not be called, got limit %d", refresher.limit)
	}
}

func TestSponsorWatchlistRefreshWorkerPropagatesRefreshError(t *testing.T) {
	refresher := &fakeSponsorWatchlistRefresher{err: errors.New("database failed")}
	worker := NewSponsorWatchlistRefreshWorker(refresher)

	err := worker.Handle(context.Background(), background.Job{
		Payload: []byte(`{"limit":500}`),
	})
	if err == nil {
		t.Fatal("expected refresh error")
	}
	if refresher.limit != 500 {
		t.Fatalf("expected refresher call with 500, got %d", refresher.limit)
	}
}
