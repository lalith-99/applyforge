package jobrecommendations

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
)

// Handlers wires the jobrecommendations Repository to HTTP routes.
type Handlers struct {
	repo *Repository
}

// NewHandlers builds jobrecommendations Handlers.
func NewHandlers(repo *Repository) *Handlers {
	return &Handlers{repo: repo}
}

// Mount registers recommendation routes onto r. Callers must apply
// auth.RequireAuth before mounting.
func (h *Handlers) Mount(r chi.Router) {
	r.Get("/recommendations", h.handleList)
}

// handleList returns a user's precomputed recommendations (Phase I) -
// a fast indexed read, since the expensive funnel already ran in the
// background whenever the user's candidate profile last changed.
func (h *Handlers) handleList(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	limit := int32(20)
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 100 {
			limit = int32(parsed)
		}
	}

	recs, err := h.repo.ListForUser(r.Context(), u.ID, limit)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not load recommendations")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"recommendations": recs})
}
