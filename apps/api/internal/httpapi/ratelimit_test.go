package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_AllowsUpToLimitThenBlocks(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		if !rl.allow("1.2.3.4") {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
	}
	if rl.allow("1.2.3.4") {
		t.Fatalf("expected 4th request to be blocked")
	}
}

func TestRateLimiter_TracksKeysIndependently(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)

	if !rl.allow("1.2.3.4") {
		t.Fatalf("expected first request from 1.2.3.4 to be allowed")
	}
	if !rl.allow("5.6.7.8") {
		t.Fatalf("expected first request from a different IP to be allowed independently")
	}
	if rl.allow("1.2.3.4") {
		t.Fatalf("expected second request from 1.2.3.4 to be blocked")
	}
}

func TestRateLimiter_Middleware_Returns429WhenExceeded(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)
	handler := rl.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	first := httptest.NewRequest(http.MethodGet, "/", nil)
	first.RemoteAddr = "9.9.9.9:1234"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, first)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first request to succeed, got %d", rec1.Code)
	}

	second := httptest.NewRequest(http.MethodGet, "/", nil)
	second.RemoteAddr = "9.9.9.9:5678" // same IP, different port
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, second)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request from the same IP to be rate limited, got %d", rec2.Code)
	}
}
