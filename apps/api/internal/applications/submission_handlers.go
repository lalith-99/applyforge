package applications

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
)

type companionLeaseRequest struct {
	WorkerID        string `json:"worker_id"`
	LeaseGeneration int64  `json:"lease_generation"`
}

type companionConfirmRequest struct {
	WorkerID        string          `json:"worker_id"`
	LeaseGeneration int64           `json:"lease_generation"`
	Receipt         json.RawMessage `json:"receipt"`
}

type companionUncertainRequest struct {
	WorkerID        string `json:"worker_id"`
	LeaseGeneration int64  `json:"lease_generation"`
	Message         string `json:"message"`
}

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

func (h *Handlers) handleClaimSubmissionIntent(w http.ResponseWriter, r *http.Request) {
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
	var req companionLeaseRequest
	if err := httpx.DecodeJSON(r, &req); err != nil || req.WorkerID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "worker_id is required")
		return
	}

	intent, err := h.svc.ClaimSubmissionIntentForUser(r.Context(), u.ID, intentID, req.WorkerID)
	if err != nil {
		writeSubmissionError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, intent)
}

func (h *Handlers) handleBeginSubmission(w http.ResponseWriter, r *http.Request) {
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
	var req companionLeaseRequest
	if err := httpx.DecodeJSON(r, &req); err != nil || req.WorkerID == "" || req.LeaseGeneration <= 0 {
		httpx.WriteError(w, http.StatusBadRequest, "worker_id and lease_generation are required")
		return
	}

	intent, err := h.svc.BeginSubmissionForUser(r.Context(), u.ID, intentID, req.WorkerID, req.LeaseGeneration)
	if err != nil {
		writeSubmissionError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, intent)
}

func (h *Handlers) handleConfirmSubmission(w http.ResponseWriter, r *http.Request) {
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
	var req companionConfirmRequest
	if err := httpx.DecodeJSON(r, &req); err != nil || req.WorkerID == "" || req.LeaseGeneration <= 0 || len(req.Receipt) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "worker_id, lease_generation, and receipt are required")
		return
	}

	intent, err := h.svc.ConfirmSubmissionForUser(r.Context(), u.ID, intentID, req.WorkerID, req.LeaseGeneration, req.Receipt)
	if err != nil {
		writeSubmissionError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, intent)
}

func (h *Handlers) handleUncertainSubmission(w http.ResponseWriter, r *http.Request) {
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
	var req companionUncertainRequest
	if err := httpx.DecodeJSON(r, &req); err != nil || req.WorkerID == "" || req.LeaseGeneration <= 0 || req.Message == "" {
		httpx.WriteError(w, http.StatusBadRequest, "worker_id, lease_generation, and message are required")
		return
	}

	intent, err := h.svc.MarkSubmissionUncertainForUser(r.Context(), u.ID, intentID, req.WorkerID, req.LeaseGeneration, req.Message)
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
	case errors.Is(err, ErrStaleSubmissionLease):
		httpx.WriteError(w, http.StatusConflict, "submission lease is stale, expired, or no longer authorized")
	case errors.Is(err, ErrInvalidSubmissionWorkerID):
		httpx.WriteError(w, http.StatusBadRequest, "worker_id is required")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "could not process submission intent")
	}
}
