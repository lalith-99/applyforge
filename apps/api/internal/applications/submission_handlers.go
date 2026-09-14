package applications

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
)

func (h *Handlers) handleCreateSubmissionIntent(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	packageID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid package id")
		return
	}

	intent, err := h.svc.CreateSubmissionIntent(r.Context(), u.ID, packageID)
	if err != nil {
		writeSubmissionError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, intent)
}

func (h *Handlers) handleGetSubmissionIntent(w http.ResponseWriter, r *http.Request) {
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

	intent, err := h.svc.GetSubmissionIntent(r.Context(), u.ID, intentID)
	if err != nil {
		writeSubmissionError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, intent)
}

func writeSubmissionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrApplicationPackageMissing), errors.Is(err, ErrSubmissionIntentNotFound), errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "application submission resource not found")
	case errors.Is(err, ErrApplicationNotReady):
		httpx.WriteError(w, http.StatusConflict, "application must be READY_TO_APPLY")
	case errors.Is(err, ErrActiveApprovalRequired):
		httpx.WriteError(w, http.StatusConflict, "an active approval for this exact application package is required")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "could not process submission intent")
	}
}
