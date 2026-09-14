package applications

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
)

type reconciliationRequest struct {
	Outcome string `json:"outcome"`
	Note    string `json:"note"`
}

func (h *Handlers) handleReconcileSubmission(w http.ResponseWriter, r *http.Request) {
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
	var req reconciliationRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	intent, err := h.svc.ReconcileSubmission(r.Context(), u.ID, intentID, req.Outcome, req.Note)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidSubmissionReconciliation):
			httpx.WriteError(w, http.StatusBadRequest, "outcome must be APPLIED or NOT_SUBMITTED")
		case errors.Is(err, ErrSubmissionIntentNotFound):
			httpx.WriteError(w, http.StatusConflict, "only an UNCERTAIN submission owned by this user can be reconciled")
		default:
			httpx.WriteError(w, http.StatusInternalServerError, "could not reconcile submission")
		}
		return
	}
	httpx.WriteJSON(w, http.StatusOK, intent)
}
