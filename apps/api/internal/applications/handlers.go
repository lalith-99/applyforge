package applications

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
)

// Handlers wires the applications Service/Repository to HTTP routes.
type Handlers struct {
	svc  *Service
	repo *Repository
}

// NewHandlers builds applications Handlers.
func NewHandlers(svc *Service, repo *Repository) *Handlers {
	return &Handlers{svc: svc, repo: repo}
}

// Mount registers applications routes onto r. Callers must apply
// auth.RequireAuth before mounting.
func (h *Handlers) Mount(r chi.Router) {
	r.Post("/applications", h.handleSave)
	r.Get("/applications", h.handleList)
	r.Get("/applications/{id}", h.handleGet)
	r.Patch("/applications/{id}", h.handleUpdate)
	r.Get("/applications/{id}/events", h.handleListEvents)
	r.Get("/application-answers", h.handleGetAnswers)
	r.Patch("/application-answers", h.handleUpdateAnswers)
}

type saveRequest struct {
	JobID           string  `json:"job_id"`
	ResumeVersionID *string `json:"resume_version_id"`
	MatchScore      *int32  `json:"match_score"`
}

func (h *Handlers) handleSave(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req saveRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	jobID, err := uuid.Parse(req.JobID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid job_id")
		return
	}
	resumeVersionID, err := parseOptionalUUID(req.ResumeVersionID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid resume_version_id")
		return
	}

	app, err := h.svc.Save(r.Context(), u.ID, jobID, resumeVersionID, req.MatchScore)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not save application")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, app)
}

func (h *Handlers) handleList(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	apps, err := h.repo.ListWithJobForUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not list applications")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, apps)
}

func (h *Handlers) handleGet(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	app, err := h.repo.GetForUser(r.Context(), id, u.ID)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "application not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, app)
}

type updateRequest struct {
	Status     *string `json:"status"`
	Notes      *string `json:"notes"`
	NextAction *string `json:"next_action"`
}

func (h *Handlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid application id")
		return
	}

	var req updateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var app Application
	if req.Status != nil {
		app, err = h.svc.ChangeStatus(r.Context(), u.ID, id, *req.Status, req.Notes)
		if err != nil {
			if errors.Is(err, ErrInvalidStatus) {
				httpx.WriteError(w, http.StatusBadRequest, "invalid status")
				return
			}
			if errors.Is(err, ErrNotFound) {
				httpx.WriteError(w, http.StatusNotFound, "application not found")
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, "could not update application status")
			return
		}
	}
	if req.Notes != nil || req.NextAction != nil {
		app, err = h.repo.UpdateNotes(r.Context(), id, u.ID, req.Notes, req.NextAction)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				httpx.WriteError(w, http.StatusNotFound, "application not found")
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, "could not update application notes")
			return
		}
	}
	if req.Status == nil && req.Notes == nil && req.NextAction == nil {
		app, err = h.repo.GetForUser(r.Context(), id, u.ID)
		if err != nil {
			httpx.WriteError(w, http.StatusNotFound, "application not found")
			return
		}
	}

	httpx.WriteJSON(w, http.StatusOK, app)
}

func (h *Handlers) handleListEvents(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid application id")
		return
	}
	if _, err := h.repo.GetForUser(r.Context(), id, u.ID); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "application not found")
		return
	}

	events, err := h.repo.ListEvents(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not list application events")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, events)
}

func (h *Handlers) handleGetAnswers(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	answers, err := h.repo.GetAnswers(r.Context(), u.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteJSON(w, http.StatusOK, Answers{UserID: u.ID, CommonAnswers: []byte(`{}`)})
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "could not load application answers")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, answers)
}

func (h *Handlers) handleUpdateAnswers(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req UpsertAnswersInput
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	answers, err := h.repo.UpsertAnswers(r.Context(), u.ID, req)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not save application answers")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, answers)
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
