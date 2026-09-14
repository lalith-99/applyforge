package applications

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
)

const companionTokenHeader = "X-ApplyForge-Companion-Token"

// CompanionUserHandlers contains session-authenticated endpoints that mint a
// narrowly scoped capability for the browser extension.
type CompanionUserHandlers struct {
	service *CompanionService
}

func NewCompanionUserHandlers(service *CompanionService) *CompanionUserHandlers {
	return &CompanionUserHandlers{service: service}
}

func (h *CompanionUserHandlers) Mount(r chi.Router) {
	r.Post("/submission-intents/{id}/companion-handoff", h.handleCreateHandoff)
}

func (h *CompanionUserHandlers) handleCreateHandoff(w http.ResponseWriter, r *http.Request) {
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
	handoff, err := h.service.CreateHandoff(r.Context(), u.ID, intentID)
	if err != nil {
		writeCompanionError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, handoff)
}

// CompanionHandlers authenticates itself with an intent-scoped bearer token;
// it is mounted outside the normal user-session group so ATS pages never need
// access to the ApplyForge session cookie.
type CompanionHandlers struct {
	service *CompanionService
}

func NewCompanionHandlers(service *CompanionService) *CompanionHandlers {
	return &CompanionHandlers{service: service}
}

func (h *CompanionHandlers) Mount(r chi.Router) {
	r.Get("/companion/submissions/{id}", h.handleBundle)
	r.Post("/companion/submissions/{id}/claim", h.handleClaim)
	r.Post("/companion/submissions/{id}/begin", h.handleBegin)
	r.Post("/companion/submissions/{id}/confirm", h.handleConfirm)
	r.Post("/companion/submissions/{id}/uncertain", h.handleUncertain)
}

func (h *CompanionHandlers) handleBundle(w http.ResponseWriter, r *http.Request) {
	intentID, ok := parseCompanionIntentID(w, r)
	if !ok {
		return
	}
	bundle, err := h.service.Bundle(r.Context(), intentID, companionToken(r))
	if err != nil {
		writeCompanionError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, bundle)
}

func (h *CompanionHandlers) handleClaim(w http.ResponseWriter, r *http.Request) {
	intentID, ok := parseCompanionIntentID(w, r)
	if !ok {
		return
	}
	var req companionLeaseRequest
	if err := httpx.DecodeJSON(r, &req); err != nil || strings.TrimSpace(req.WorkerID) == "" {
		httpx.WriteError(w, http.StatusBadRequest, "worker_id is required")
		return
	}
	intent, err := h.service.Claim(r.Context(), intentID, companionToken(r), req.WorkerID)
	if err != nil {
		writeCompanionError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, intent)
}

func (h *CompanionHandlers) handleBegin(w http.ResponseWriter, r *http.Request) {
	intentID, ok := parseCompanionIntentID(w, r)
	if !ok {
		return
	}
	var req companionLeaseRequest
	if err := httpx.DecodeJSON(r, &req); err != nil || req.WorkerID == "" || req.LeaseGeneration <= 0 {
		httpx.WriteError(w, http.StatusBadRequest, "worker_id and lease_generation are required")
		return
	}
	intent, err := h.service.Begin(r.Context(), intentID, companionToken(r), req.WorkerID, req.LeaseGeneration)
	if err != nil {
		writeCompanionError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, intent)
}

func (h *CompanionHandlers) handleConfirm(w http.ResponseWriter, r *http.Request) {
	intentID, ok := parseCompanionIntentID(w, r)
	if !ok {
		return
	}
	var req companionConfirmRequest
	if err := httpx.DecodeJSON(r, &req); err != nil || req.WorkerID == "" || req.LeaseGeneration <= 0 || len(req.Receipt) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "worker_id, lease_generation, and receipt are required")
		return
	}
	intent, err := h.service.Confirm(r.Context(), intentID, companionToken(r), req.WorkerID, req.LeaseGeneration, req.Receipt)
	if err != nil {
		writeCompanionError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, intent)
}

func (h *CompanionHandlers) handleUncertain(w http.ResponseWriter, r *http.Request) {
	intentID, ok := parseCompanionIntentID(w, r)
	if !ok {
		return
	}
	var req companionUncertainRequest
	if err := httpx.DecodeJSON(r, &req); err != nil || req.WorkerID == "" || req.LeaseGeneration <= 0 || strings.TrimSpace(req.Message) == "" {
		httpx.WriteError(w, http.StatusBadRequest, "worker_id, lease_generation, and message are required")
		return
	}
	intent, err := h.service.Uncertain(r.Context(), intentID, companionToken(r), req.WorkerID, req.LeaseGeneration, req.Message)
	if err != nil {
		writeCompanionError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, intent)
}

func parseCompanionIntentID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	intentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid submission intent id")
		return uuid.Nil, false
	}
	return intentID, true
}

func companionToken(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get(companionTokenHeader))
}

func writeCompanionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrCompanionUnauthorized):
		httpx.WriteError(w, http.StatusUnauthorized, "invalid or expired companion token")
	case errors.Is(err, ErrSubmissionIntentNotFound), errors.Is(err, ErrApplicationPackageMissing), errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "submission resource not found")
	case errors.Is(err, ErrStaleSubmissionLease), errors.Is(err, ErrActiveApprovalRequired):
		httpx.WriteError(w, http.StatusConflict, "submission is no longer authorized or the execution lease is stale")
	default:
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid receipt JSON")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "could not process companion request")
	}
}
