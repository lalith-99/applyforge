package applications

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
)

// MountPublic registers the self-authenticated companion routes. httpapi mounts
// this method outside the normal user-session middleware; every route still
// requires a short-lived intent-scoped companion capability.
func (h *Handlers) MountPublic(r chi.Router) {
	NewCompanionHandlers(NewCompanionService(h.svc, h.repo)).Mount(r)
}

func (h *Handlers) handleCreateCompanionHandoff(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	intentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid submission intent id")
		return
	}

	handoff, err := NewCompanionService(h.svc, h.repo).CreateHandoff(r.Context(), u.ID, intentID)
	if err != nil {
		writeCompanionError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, handoff)
}
