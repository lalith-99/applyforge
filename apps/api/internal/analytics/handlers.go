package analytics

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
)

// Handlers wires the analytics Service to HTTP routes.
type Handlers struct {
	svc *Service
}

// NewHandlers builds analytics Handlers.
func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

// Mount registers analytics routes onto r. Callers must apply
// auth.RequireAuth before mounting.
func (h *Handlers) Mount(r chi.Router) {
	r.Get("/analytics/dashboard", h.handleDashboard)
}

func (h *Handlers) handleDashboard(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	dashboard, err := h.svc.Dashboard(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not load analytics dashboard")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dashboard)
}
