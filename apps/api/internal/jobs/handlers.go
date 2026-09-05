package jobs

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
)

// Handlers wires the jobs Repository/IngestionService to HTTP routes.
type Handlers struct {
	repo *Repository
	svc  *IngestionService
}

// NewHandlers builds jobs Handlers.
func NewHandlers(repo *Repository, svc *IngestionService) *Handlers {
	return &Handlers{repo: repo, svc: svc}
}

// Mount registers job routes onto r. Callers must apply auth.RequireAuth
// before mounting.
func (h *Handlers) Mount(r chi.Router) {
	r.Get("/jobs", h.handleList)
	r.Get("/jobs/{id}", h.handleGet)
	r.Post("/admin/job-sources/sync", h.handleSync)
}

func (h *Handlers) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var postedAfter *time.Time
	if within := q.Get("posted_within"); within != "" {
		if d, err := time.ParseDuration(within); err == nil {
			t := time.Now().Add(-d)
			postedAfter = &t
		}
	}

	limit := int32(20)
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 {
		limit = int32(l)
	}
	offset := int32(0)
	if o, err := strconv.Atoi(q.Get("offset")); err == nil && o > 0 {
		offset = int32(o)
	}

	jobsList, total, err := h.repo.List(r.Context(), ListFilter{
		Search:         q.Get("search"),
		RemoteType:     q.Get("remote_type"),
		EmploymentType: q.Get("employment_type"),
		PostedAfter:    postedAfter,
		Sort:           q.Get("sort"),
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not list jobs")
		return
	}

	items := make([]map[string]any, 0, len(jobsList))
	for _, j := range jobsList {
		items = append(items, toSummary(j))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *Handlers) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	job, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "job not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toDetail(job))
}

func (h *Handlers) handleSync(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.SyncAll(r.Context()); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "sync failed")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "synced"})
}

func toSummary(j Job) map[string]any {
	return map[string]any{
		"id":               j.ID,
		"source":           j.Source,
		"company_name":     j.CompanyName,
		"title":            j.Title,
		"normalized_title": j.NormalizedTitle,
		"location_text":    j.LocationText,
		"remote_type":      j.RemoteType,
		"employment_type":  j.EmploymentType,
		"salary_min":       j.SalaryMin,
		"salary_max":       j.SalaryMax,
		"salary_currency":  j.SalaryCurrency,
		"apply_url":        j.ApplyURL,
		"posted_at":        j.PostedAt,
		"first_seen_at":    j.FirstSeenAt,
	}
}

func toDetail(j Job) map[string]any {
	detail := toSummary(j)
	detail["description"] = j.Description
	detail["source_url"] = j.SourceURL
	detail["status"] = j.Status
	return detail
}
