package httpapi

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// rateLimiter is a simple in-memory, per-key fixed-window rate limiter.
// It's intentionally not distributed — fine for a single-instance API
// service, but would need a shared store (e.g. Redis) behind a load
// balancer with multiple replicas (see DECISIONS.md, Phase 12).
type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitorState
	limit    int
	window   time.Duration
}

type visitorState struct {
	count     int
	windowEnd time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		visitors: make(map[string]*visitorState),
		limit:    limit,
		window:   window,
	}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	// Opportunistically prune expired entries so the map doesn't grow
	// unbounded; cheap relative to the request itself since most maps
	// stay small (unique client IPs in the current window).
	if len(rl.visitors) > 10000 {
		for k, v := range rl.visitors {
			if now.After(v.windowEnd) {
				delete(rl.visitors, k)
			}
		}
	}

	v, ok := rl.visitors[key]
	if !ok || now.After(v.windowEnd) {
		rl.visitors[key] = &visitorState{count: 1, windowEnd: now.Add(rl.window)}
		return true
	}
	if v.count >= rl.limit {
		return false
	}
	v.count++
	return true
}

// middleware rejects requests over the configured rate with 429, keyed by
// client IP (relies on middleware.RealIP already having normalized
// r.RemoteAddr earlier in the chain).
func (rl *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.RemoteAddr
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			key = host
		}
		if !rl.allow(key) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many requests"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
