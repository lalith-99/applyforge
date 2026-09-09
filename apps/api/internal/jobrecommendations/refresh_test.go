package jobrecommendations

import (
	"context"
	"testing"
	"time"
)

func TestRunCatalogRefreshDebouncer_CoalescesBurst(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changes := make(chan struct{}, 4)
	refreshed := make(chan struct{}, 4)

	go RunCatalogRefreshDebouncer(ctx, changes, 40*time.Millisecond, func(context.Context) error {
		refreshed <- struct{}{}
		return nil
	})

	changes <- struct{}{}
	time.Sleep(20 * time.Millisecond)
	changes <- struct{}{}
	time.Sleep(20 * time.Millisecond)
	changes <- struct{}{}

	select {
	case <-refreshed:
		t.Fatal("refresh fired before the quiet period elapsed")
	case <-time.After(25 * time.Millisecond):
	}

	select {
	case <-refreshed:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected one refresh after the catalog became quiet")
	}

	select {
	case <-refreshed:
		t.Fatal("expected the burst to coalesce into a single refresh")
	case <-time.After(30 * time.Millisecond):
	}
}
