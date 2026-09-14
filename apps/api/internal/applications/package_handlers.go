package applications

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
)

type packageApprovalRequest struct {
	ConfirmationVersion string `json:"confirmation_version"`
	ConfirmationText    string `json:"confirmation_text"`
}

func (h *Handlers) handleBuildPackage(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	applicationID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	pkg, err := h.svc.BuildApplicationPackage(r.Context(), u.ID, applicationID)
	if err != nil {
		writePackageError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, pkg)
}

func (h *Handlers) handleGetPackage(w http.ResponseWriter, r *http.Request) {
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

	pkg, err := h.svc.GetApplicationPackage(r.Context(), u.ID, packageID)
	if err != nil {
		writePackageError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, pkg)
}

func (h *Handlers) handleApprovePackage(w http.ResponseWriter, r *http.Request) {
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
	var req packageApprovalRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	approval, err := h.svc.ApproveApplicationPackage(r.Context(), u.ID, packageID, req.ConfirmationVersion, req.ConfirmationText)
	if err != nil {
		writePackageError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, approval)
}

func (h *Handlers) handleRevokePackageApproval(w http.ResponseWriter, r *http.Request) {
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

	approval, err := h.svc.RevokeApplicationPackageApproval(r.Context(), u.ID, packageID)
	if err != nil {
		writePackageError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, approval)
}

func writePackageError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrApplicationPackageMissing):
		httpx.WriteError(w, http.StatusNotFound, "application package not found")
	case errors.Is(err, ErrApplicationNotReady):
		httpx.WriteError(w, http.StatusConflict, "application must be READY_TO_APPLY")
	case errors.Is(err, ErrMissingResumeVersion):
		httpx.WriteError(w, http.StatusConflict, "application needs a job-scoped resume version")
	case errors.Is(err, ErrResumeVersionMismatch):
		httpx.WriteError(w, http.StatusConflict, "resume version does not match this application")
	case errors.Is(err, ErrMissingDestination):
		httpx.WriteError(w, http.StatusConflict, "job has no safe HTTPS application destination")
	case errors.Is(err, ErrInvalidApproval):
		httpx.WriteError(w, http.StatusBadRequest, "approval confirmation does not match the required text and version")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "could not process application package")
	}
}
