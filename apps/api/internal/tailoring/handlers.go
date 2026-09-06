package tailoring

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/background"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
)

// Handlers wires the tailoring Service/Repository to HTTP routes.
type Handlers struct {
	svc   *Service
	repo  *Repository
	queue *background.Queue
}

// NewHandlers builds tailoring Handlers.
func NewHandlers(svc *Service, repo *Repository, queue *background.Queue) *Handlers {
	return &Handlers{svc: svc, repo: repo, queue: queue}
}

// Mount registers tailoring routes onto r. Callers must apply
// auth.RequireAuth before mounting.
func (h *Handlers) Mount(r chi.Router) {
	r.Post("/jobs/{jobId}/tailor", h.handleTailor)
	r.Get("/tailoring/{id}", h.handleGet)
	r.Patch("/tailoring/{id}/suggestions/{suggestionId}", h.handleUpdateSuggestion)
	r.Post("/tailoring/{id}/approve-all", h.handleApproveAll)
}

type tailorRequest struct {
	ResumeID string `json:"resume_id"`
	Mode     string `json:"mode"`
}

func (h *Handlers) handleTailor(w http.ResponseWriter, r *http.Request) {
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

	var req tailorRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resumeID, err := uuid.Parse(req.ResumeID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid resume_id")
		return
	}
	mode := req.Mode
	if mode == "" {
		mode = ModeGrowth
	}
	if mode != ModeStrict && mode != ModeGrowth && mode != ModeMaxMatch {
		httpx.WriteError(w, http.StatusBadRequest, "mode must be STRICT, GROWTH, or MAX_MATCH")
		return
	}

	run, err := h.svc.CreateQueuedRun(r.Context(), u.ID, jobID, resumeID, mode)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "could not start tailoring run")
		return
	}
	if err := h.queue.Enqueue(r.Context(), JobTypeProcess, ProcessPayload{RunID: run.ID.String()}, 3); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not queue tailoring run")
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, toRunDetail(run, nil))
}

func (h *Handlers) handleGet(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	runID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid tailoring run id")
		return
	}

	run, err := h.repo.GetRunForUser(r.Context(), runID, u.ID)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "tailoring run not found")
		return
	}
	suggestions, err := h.repo.ListSuggestions(r.Context(), runID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not load suggestions")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toRunDetail(run, suggestions))
}

type updateSuggestionRequest struct {
	Status     string  `json:"status"`
	EditedText *string `json:"edited_text"`
}

func (h *Handlers) handleUpdateSuggestion(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	runID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid tailoring run id")
		return
	}
	suggestionID, err := uuid.Parse(chi.URLParam(r, "suggestionId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid suggestion id")
		return
	}

	if _, err := h.repo.GetRunForUser(r.Context(), runID, u.ID); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "tailoring run not found")
		return
	}

	var req updateSuggestionRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	switch req.Status {
	case StatusApproved, StatusEdited, StatusRejected, StatusPending:
	default:
		httpx.WriteError(w, http.StatusBadRequest, "status must be PENDING, APPROVED, EDITED, or REJECTED")
		return
	}

	updated, err := h.repo.UpdateSuggestionStatus(r.Context(), suggestionID, runID, req.Status, req.EditedText)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not update suggestion")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, updated)
}

func (h *Handlers) handleApproveAll(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	runID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid tailoring run id")
		return
	}
	if _, err := h.repo.GetRunForUser(r.Context(), runID, u.ID); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "tailoring run not found")
		return
	}

	if err := h.repo.ApproveAllPending(r.Context(), runID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not approve suggestions")
		return
	}
	suggestions, err := h.repo.ListSuggestions(r.Context(), runID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not load suggestions")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, suggestions)
}

func toRunDetail(run Run, suggestions []Suggestion) map[string]any {
	var summarySuggestion json.RawMessage = run.SummarySuggestion
	return map[string]any{
		"id":                     run.ID,
		"job_id":                 run.JobID,
		"resume_id":              run.ResumeID,
		"mode":                   run.Mode,
		"status":                 run.Status,
		"summary_suggestion":     summarySuggestion,
		"alignment_score_before": run.AlignmentScoreBefore,
		"alignment_score_after":  run.AlignmentScoreAfter,
		"keyword_coverage":       run.KeywordCoverage,
		"critic_result":          run.CriticResult,
		"created_at":             run.CreatedAt,
		"completed_at":           run.CompletedAt,
		"suggestions":            suggestions,
	}
}
