// Package httpapi assembles the HTTP router and handlers for the API service.
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Pinger checks database connectivity for readiness probes.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Mounter registers a set of routes onto a chi.Router. auth.Handlers,
// profile.Handlers, and preferences.Handlers all satisfy this via their
// Mount method.
type Mounter interface {
	Mount(r chi.Router)
}

// Config bundles everything NewRouter needs to wire up routes. It exists so
// main.go doesn't need to know internal package import paths directly cause
// cyclic-import risk between httpapi and the domain packages.
type Config struct {
	DB          Pinger
	WebBaseURL  string
	RequireAuth func(http.Handler) http.Handler
	Auth        Mounter
	Authed      []Mounter
}

// NewRouter builds the chi router with health/readiness endpoints and the
// /api/v1 product API.
func NewRouter(cfg Config) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.WebBaseURL},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Rate limiting (see DECISIONS.md, Phase 12): a stricter per-IP limit on
	// /auth guards against credential-stuffing/brute-force, and a more
	// generous global limit protects the rest of the API from abuse.
	authRateLimiter := newRateLimiter(20, time.Minute)
	apiRateLimiter := newRateLimiter(300, time.Minute)

	r.Get("/health", handleHealth)
	r.Get("/ready", handleReady(cfg.DB))

	r.Route("/api/v1", func(r chi.Router) {
		if cfg.Auth != nil {
			r.Route("/auth", func(r chi.Router) {
				r.Use(authRateLimiter.middleware)
				cfg.Auth.Mount(r)
			})
		}

		r.Group(func(r chi.Router) {
			r.Use(apiRateLimiter.middleware)
			if cfg.RequireAuth != nil {
				r.Use(cfg.RequireAuth)
			}
			for _, m := range cfg.Authed {
				m.Mount(r)
			}
		})
	})

	return r
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleReady(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "not_ready",
				"error":  "database unreachable",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
