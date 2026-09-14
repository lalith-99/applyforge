package applications

import (
	"net/http"

	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
)

func (h *Handlers) handleListSubmissionIntents(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	intents, err := h.svc.ListLatestSubmissionIntentsForUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not list submission states")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, intents)
}
