package matching

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
)

// Handlers wires the matching Service to HTTP routes.
type Handlers struct {
	svc *Service
}

// NewHandlers builds matching Handlers.
func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

// Mount registers matching routes onto r. Callers must apply auth.RequireAuth
// before mounting.
func (h *Handlers) Mount(r chi.Router) {
	r.Get("/jobs/{id}/match", h.handleMatch)
}

func (h *Handlers) handleMatch(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	jobID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	result, err := h.svc.Match(r.Context(), jobID, u.ID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "could not compute job match")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}
