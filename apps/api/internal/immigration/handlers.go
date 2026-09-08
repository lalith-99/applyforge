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
	r.Post("/admin/immigration/evidence/import", h.handleImport)\n\tr.Post("/admin/immigration/watchlist/refresh", h.handleWatchlistRefresh)\n\tr.Get("/admin/immigration/watchlist/summary", h.handleWatchlistSummary)
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
