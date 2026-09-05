package learning

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
)

// Handlers wires the learning Service to HTTP routes.
type Handlers struct {
	svc *Service
}

// NewHandlers builds learning Handlers.
func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

// Mount registers learning routes onto r. Callers must apply
// auth.RequireAuth before mounting.
func (h *Handlers) Mount(r chi.Router) {
	r.Get("/skills/{skill}/quick-prep", h.handleQuickPrep)
	r.Post("/defend-bullet", h.handleDefendBullet)
	r.Post("/jobs/{jobId}/learning-plan", h.handleLearningPlan)
	r.Post("/jobs/{jobId}/make-me-qualified", h.handleMakeMeQualified)
}

func (h *Handlers) handleQuickPrep(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	skill := chi.URLParam(r, "skill")
	if skill == "" {
		httpx.WriteError(w, http.StatusBadRequest, "skill is required")
		return
	}

	module, err := h.svc.QuickPrep(r.Context(), u.ID, skill)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "could not generate quick prep")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, module)
}

// defendBulletRequest carries the bullet text and its associated skills
// directly (rather than resolving a suggestion/experience ID server-side),
// since the caller (tailoring suggestion card or resume experience row)
// already has both values on hand client-side.
type defendBulletRequest struct {
	BulletText string   `json:"bullet_text"`
	Skills     []string `json:"skills"`
}

func (h *Handlers) handleDefendBullet(w http.ResponseWriter, r *http.Request) {
	_, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req defendBulletRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.BulletText == "" {
		httpx.WriteError(w, http.StatusBadRequest, "bullet_text is required")
		return
	}

	resp, err := h.svc.DefendBullet(r.Context(), req.BulletText, req.Skills)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "could not generate defend-bullet questions")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handlers) handleLearningPlan(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	jobID, err := uuid.Parse(chi.URLParam(r, "jobId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	plan, err := h.svc.LearningPlan(r.Context(), u.ID, jobID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "could not generate learning plan")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, plan)
}

func (h *Handlers) handleMakeMeQualified(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	jobID, err := uuid.Parse(chi.URLParam(r, "jobId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	result, err := h.svc.MakeMeQualified(r.Context(), u.ID, jobID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "could not analyze qualification gap")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}
