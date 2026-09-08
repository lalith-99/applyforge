package immigration

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
)

type Handlers struct {
	repo       *Repository
	adminToken string
}

func NewHandlers(repo *Repository, adminToken string) *Handlers {
	return &Handlers{repo: repo, adminToken: strings.TrimSpace(adminToken)}
}

func (h *Handlers) Mount(r chi.Router) {
	if h.adminToken == "" {
		return
	}
	r.Post("/admin/immigration/evidence/import", h.handleImport)
	r.Post("/admin/immigration/watchlist/refresh", h.handleWatchlistRefresh)
	r.Get("/admin/immigration/watchlist/summary", h.handleWatchlistSummary)
}

func (h *Handlers) authorized(r *http.Request) bool {
	provided := r.Header.Get("X-ApplyForge-Admin-Token")
	return subtle.ConstantTimeCompare([]byte(provided), []byte(h.adminToken)) == 1
}

func (h *Handlers) handleImport(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		httpx.WriteError(w, http.StatusForbidden, "admin authorization required")
		return
	}

	var batch ImportBatch
	if err := httpx.DecodeJSON(r, &batch); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid immigration evidence batch")
		return
	}
	imported, err := h.repo.Import(r.Context(), batch)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"imported":       imported,
		"source_release": batch.SourceRelease,
		"rows_received":  len(batch.Rows),
		"max_batch_size": strconv.Itoa(2000),
	})
}


func (h *Handlers) handleWatchlistRefresh(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		httpx.WriteError(w, http.StatusForbidden, "admin authorization required")
		return
	}

	limit := 10000
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 50000 {
			httpx.WriteError(w, http.StatusBadRequest, "limit must be an integer between 1 and 50000")
			return
		}
		limit = parsed
	}

	count, err := h.repo.RefreshSponsorWatchlist(r.Context(), limit)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"watchlist_companies": count,
		"requested_limit":     limit,
		"primary_signal":      "recent certified H-1B LCA activity",
		"secondary_signal":    "recent PERM activity (ranking metadata only)",
	})
}

func (h *Handlers) handleWatchlistSummary(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		httpx.WriteError(w, http.StatusForbidden, "admin authorization required")
		return
	}

	summary, err := h.repo.GetSponsorWatchlistSummary(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, summary)
}
