package immigration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeJobEnqueuer struct {
	jobType     string
	payload     any
	maxAttempts int32
	err         error
}

func (f *fakeJobEnqueuer) Enqueue(
	_ context.Context,
	jobType string,
	payload any,
	maxAttempts int32,
) error {
	f.jobType = jobType
	f.payload = payload
	f.maxAttempts = maxAttempts
	return f.err
}

func TestHandleWatchlistRefreshQueuesBackgroundJob(t *testing.T) {
	queue := &fakeJobEnqueuer{}
	h := NewHandlers(nil, "test-admin-token").WithQueue(queue)

	req := httptest.NewRequest(
		http.MethodPost,
		"/admin/immigration/watchlist/refresh?limit=1234",
		nil,
	)
	req.Header.Set("X-ApplyForge-Admin-Token", "test-admin-token")
	rec := httptest.NewRecorder()

	h.handleWatchlistRefresh(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected %d, got %d: %s", http.StatusAccepted, rec.Code, rec.Body.String())
	}
	if queue.jobType != JobTypeRefreshSponsorWatchlist {
		t.Fatalf("unexpected job type %q", queue.jobType)
	}
	payload, ok := queue.payload.(RefreshSponsorWatchlistPayload)
	if !ok {
		t.Fatalf("unexpected payload type %T", queue.payload)
	}
	if payload.Limit != 1234 {
		t.Fatalf("expected limit 1234, got %d", payload.Limit)
	}
	if queue.maxAttempts != 3 {
		t.Fatalf("expected 3 max attempts, got %d", queue.maxAttempts)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "queued" {
		t.Fatalf("expected queued status, got %#v", body["status"])
	}
}

func TestHandleWatchlistRefreshRejectsInvalidLimitBeforeEnqueue(t *testing.T) {
	queue := &fakeJobEnqueuer{}
	h := NewHandlers(nil, "test-admin-token").WithQueue(queue)

	req := httptest.NewRequest(
		http.MethodPost,
		"/admin/immigration/watchlist/refresh?limit=50001",
		nil,
	)
	req.Header.Set("X-ApplyForge-Admin-Token", "test-admin-token")
	rec := httptest.NewRecorder()

	h.handleWatchlistRefresh(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if queue.jobType != "" {
		t.Fatalf("invalid request unexpectedly enqueued %q", queue.jobType)
	}
}

func TestHandleWatchlistRefreshReturnsUnavailableWithoutQueue(t *testing.T) {
	h := NewHandlers(nil, "test-admin-token")

	req := httptest.NewRequest(
		http.MethodPost,
		"/admin/immigration/watchlist/refresh",
		nil,
	)
	req.Header.Set("X-ApplyForge-Admin-Token", "test-admin-token")
	rec := httptest.NewRecorder()

	h.handleWatchlistRefresh(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestHandleWatchlistRefreshReturnsErrorWhenEnqueueFails(t *testing.T) {
	queue := &fakeJobEnqueuer{err: errors.New("queue unavailable")}
	h := NewHandlers(nil, "test-admin-token").WithQueue(queue)

	req := httptest.NewRequest(
		http.MethodPost,
		"/admin/immigration/watchlist/refresh",
		nil,
	)
	req.Header.Set("X-ApplyForge-Admin-Token", "test-admin-token")
	rec := httptest.NewRecorder()

	h.handleWatchlistRefresh(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}
