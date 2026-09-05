package account

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
	"github.com/lalithlochan/applyforge/apps/api/internal/resume"
)

// Handlers wires the account Service to HTTP routes.
type Handlers struct {
	svc          *Service
	secureCookie bool
}

// NewHandlers builds account Handlers. secureCookie must match the value
// passed to auth.NewHandlers so the session cookie is cleared consistently.
func NewHandlers(svc *Service, secureCookie bool) *Handlers {
	return &Handlers{svc: svc, secureCookie: secureCookie}
}

// Mount registers account routes onto r. Callers must apply
// auth.RequireAuth before mounting.
func (h *Handlers) Mount(r chi.Router) {
	r.Delete("/resumes/{id}", h.handleDeleteResume)
	r.Delete("/account", h.handleDeleteAccount)
}

func (h *Handlers) handleDeleteResume(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid resume id")
		return
	}

	if err := h.svc.DeleteResume(r.Context(), u.ID, id); err != nil {
		if errors.Is(err, resume.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "resume not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "could not delete resume")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	if err := h.svc.DeleteAccount(r.Context(), u.ID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not delete account")
		return
	}

	auth.ClearSessionCookie(w, h.secureCookie)
	w.WriteHeader(http.StatusNoContent)
}
