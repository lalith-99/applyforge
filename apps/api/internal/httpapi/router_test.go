package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(ctx context.Context) error { return f.err }

func TestHealth(t *testing.T) {
	router := NewRouter(Config{DB: fakePinger{}, WebBaseURL: "http://localhost:3000"})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestReady_DatabaseUp(t *testing.T) {
	router := NewRouter(Config{DB: fakePinger{}, WebBaseURL: "http://localhost:3000"})
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestReady_DatabaseDown(t *testing.T) {
	router := NewRouter(Config{DB: fakePinger{err: context.DeadlineExceeded}, WebBaseURL: "http://localhost:3000"})
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}
}
