package resumeversion

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
	"github.com/lalithlochan/applyforge/apps/api/internal/resume"
)

// Handlers wires the resumeversion Service/Repository to HTTP routes.
type Handlers struct {
	svc        *Service
	repo       *Repository
	resumeRepo *resume.Repository
}

// NewHandlers builds resumeversion Handlers.
func NewHandlers(svc *Service, repo *Repository, resumeRepo *resume.Repository) *Handlers {
	return &Handlers{svc: svc, repo: repo, resumeRepo: resumeRepo}
}

// Mount registers resumeversion routes onto r. Callers must apply
// auth.RequireAuth before mounting.
func (h *Handlers) Mount(r chi.Router) {
	r.Post("/resumes/{resumeId}/versions", h.handleGenerate)
	r.Get("/resumes/{resumeId}/versions", h.handleList)
	r.Get("/resume-versions/{id}", h.handleGet)
	r.Get("/resume-versions/{id}/download", h.handleDownload)
}

type generateRequest struct {
	JobID          *string `json:"job_id"`
	TailoringRunID *string `json:"tailoring_run_id"`
}

func (h *Handlers) handleGenerate(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	resumeID, err := uuid.Parse(chi.URLParam(r, "resumeId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid resume id")
		return
	}

	var req generateRequest
	if r.ContentLength != 0 {
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	jobID, err := parseOptionalUUID(req.JobID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid job_id")
		return
	}
	tailoringRunID, err := parseOptionalUUID(req.TailoringRunID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid tailoring_run_id")
		return
	}

	version, err := h.svc.GenerateVersion(r.Context(), u.ID, resumeID, jobID, tailoringRunID)
	if err != nil {
		if errors.Is(err, ErrResumeNotParsed) {
			httpx.WriteError(w, http.StatusConflict, "resume has not finished parsing")
			return
		}
		httpx.WriteError(w, http.StatusBadGateway, "could not generate resume version")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, version)
}

func (h *Handlers) handleList(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	resumeID, err := uuid.Parse(chi.URLParam(r, "resumeId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid resume id")
		return
	}

	if _, err := h.resumeRepo.Get(r.Context(), resumeID, u.ID); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "resume not found")
		return
	}

	versions, err := h.repo.ListForResume(r.Context(), resumeID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not list resume versions")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, versions)
}

func (h *Handlers) handleGet(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid resume version id")
		return
	}

	version, err := h.repo.GetForUser(r.Context(), id, u.ID)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "resume version not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, version)
}

func (h *Handlers) handleDownload(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid resume version id")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "pdf"
	}

	data, contentType, filename, err := h.svc.Download(r.Context(), id, u.ID, format)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			httpx.WriteError(w, http.StatusNotFound, "document not found")
		case errors.Is(err, ErrUnknownFormat):
			httpx.WriteError(w, http.StatusBadRequest, "format must be pdf or docx")
		default:
			httpx.WriteError(w, http.StatusInternalServerError, "could not download document")
		}
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func parseOptionalUUID(s *string) (*uuid.UUID, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}
