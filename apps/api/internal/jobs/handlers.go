package jobs

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobrequirements"
	"github.com/lalithlochan/applyforge/apps/api/internal/preferences"
)

// Handlers wires the jobs Repository/IngestionService to HTTP routes.
type Handlers struct {
	repo           *Repository
	svc            *IngestionService
	requirements   *jobrequirements.Service
	preferences    *preferences.Repository
	adminSyncToken string
}

// NewHandlers builds jobs Handlers. requirements may be nil if JD parsing
// (Phase 4) isn't wired up yet; the job detail response simply omits it.
func NewHandlers(repo *Repository, svc *IngestionService, requirements *jobrequirements.Service) *Handlers {
	return &Handlers{repo: repo, svc: svc, requirements: requirements}
}

// WithAdminSyncToken enables the manual global source-sync endpoint. When the
// token is empty the route is not mounted at all; normal scheduled ingestion
// is unaffected.
func (h *Handlers) WithAdminSyncToken(token string) *Handlers {
	h.adminSyncToken = strings.TrimSpace(token)
	return h
}

// WithPreferences enables user-specific catalog hard filters such as explicit
// sponsorship denials for H-1B candidates.
func (h *Handlers) WithPreferences(repo *preferences.Repository) *Handlers {
	h.preferences = repo
	return h
}

// Mount registers job routes onto r. Callers must apply auth.RequireAuth
// before mounting.
func (h *Handlers) Mount(r chi.Router) {
	r.Get("/jobs", h.handleList)
	r.Get("/jobs/{id}", h.handleGet)
	if h.adminSyncToken != "" {
		r.Post("/admin/job-sources/sync", h.handleSync)
		r.Post("/admin/jobs/backfill", h.handleCatalogBackfill)
		r.Get("/admin/job-sources/health", h.handleSourceHealth)
	}
}

func (h *Handlers) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	excludeSponsorshipDenied := false
	requireRecentH1BHistory := false
	preferenceEmploymentType := ""
	if h.preferences != nil {
		if user, ok := auth.UserFromContext(r.Context()); ok {
			prefs, err := h.preferences.Get(r.Context(), user.ID)
			if err == nil {
				excludeSponsorshipDenied = preferences.RequiresH1BSupport(prefs)
				requireRecentH1BHistory = excludeSponsorshipDenied
				if len(prefs.EmploymentTypes) == 1 && normalizeEmploymentType(prefs.EmploymentTypes[0]) == "FullTime" {
					preferenceEmploymentType = "FullTime"
				}
			} else if !errors.Is(err, preferences.ErrNotFound) {
				httpx.WriteError(w, http.StatusInternalServerError, "could not load job preferences")
				return
			}
		}
	}
	countryCode := strings.ToUpper(strings.TrimSpace(q.Get("country")))
	location := q.Get("location")
	if countryCode == "" {
		countryCode = "US"
	}
	if countryCode == "US" && usCountryTokens[strings.ToLower(strings.TrimSpace(location))] {
		location = ""
	}

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

	employmentType := normalizeEmploymentType(q.Get("employment_type"))
	if employmentType == "" {
		employmentType = preferenceEmploymentType
	}

	jobsList, total, err := h.repo.List(r.Context(), ListFilter{
		Search:                   q.Get("search"),
		RemoteType:               q.Get("remote_type"),
		EmploymentType:           employmentType,
		PostedAfter:              postedAfter,
		Location:                 location,
		CountryCode:              countryCode,
		ExcludeSponsorshipDenied: excludeSponsorshipDenied,
		RequireRecentH1BHistory:  requireRecentH1BHistory,
		Sort:                     q.Get("sort"),
		Limit:                    limit,
		Offset:                   offset,
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

	detail := toDetail(job)
	if h.requirements != nil {
		reqs, err := h.requirements.GetOrParse(r.Context(), job.ID, job.Title, job.Description, job.ContentHash)
		if err != nil {
			httpx.WriteError(w, http.StatusBadGateway, "could not analyze job requirements")
			return
		}
		detail["requirements"] = reqs
	}
	httpx.WriteJSON(w, http.StatusOK, detail)
}

func (h *Handlers) adminAuthorized(r *http.Request) bool {
	provided := r.Header.Get("X-ApplyForge-Admin-Token")
	return subtle.ConstantTimeCompare([]byte(provided), []byte(h.adminSyncToken)) == 1
}

func (h *Handlers) handleSync(w http.ResponseWriter, r *http.Request) {
	if !h.adminAuthorized(r) {
		httpx.WriteError(w, http.StatusForbidden, "admin authorization required")
		return
	}
	if err := h.svc.EnqueueSyncTasks(r.Context()); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "sync failed")
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func (h *Handlers) handleCatalogBackfill(w http.ResponseWriter, r *http.Request) {
	if !h.adminAuthorized(r) {
		httpx.WriteError(w, http.StatusForbidden, "admin authorization required")
		return
	}
	if h.svc == nil || h.svc.queue == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, "catalog backfill queue is unavailable")
		return
	}

	batchSize := 250
	if raw := r.URL.Query().Get("batch_size"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 1000 {
			batchSize = parsed
		}
	}
	if err := h.svc.queue.Enqueue(
		r.Context(),
		JobTypeCatalogBackfill,
		CatalogBackfillPayload{BatchSize: batchSize},
		3,
	); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not queue catalog backfill")
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]any{
		"status":     "queued",
		"batch_size": batchSize,
	})
}

func (h *Handlers) handleSourceHealth(w http.ResponseWriter, r *http.Request) {
	if !h.adminAuthorized(r) {
		httpx.WriteError(w, http.StatusForbidden, "admin authorization required")
		return
	}

	limit := 200
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	sources, err := h.repo.ListSourceHealth(r.Context(), limit)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not load source health")
		return
	}
	catalog, err := h.repo.GetCatalogHealth(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not load catalog health")
		return
	}
	queue, err := h.repo.GetQueueHealth(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not load queue health")
		return
	}
	ai, err := h.repo.GetAIUsageHealth(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not load AI usage health")
		return
	}
	market, err := h.repo.GetMarketCoverageHealth(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not load market coverage health")
		return
	}
	discovery, err := h.repo.GetSourceDiscoveryHealth(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not load source discovery health")
		return
	}
	sponsorCoverage, err := h.repo.GetSponsorCoverageHealth(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not load sponsor coverage health")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"catalog":          catalog,
		"market":           market,
		"discovery":        discovery,
		"sponsor_coverage": sponsorCoverage,
		"queue":            queue,
		"ai":               ai,
		"sources":          sources,
	})
}

func toSummary(j Job) map[string]any {
	return map[string]any{
		"id":                  j.ID,
		"source":              j.Source,
		"company_name":        j.CompanyName,
		"title":               j.Title,
		"normalized_title":    j.NormalizedTitle,
		"country":             j.Country,
		"state":               j.State,
		"city":                j.City,
		"location_text":       j.LocationText,
		"country_code":        j.CountryCode,
		"state_code":          j.StateCode,
		"workplace_type":      j.WorkplaceType,
		"remote_scope":        j.RemoteScope,
		"location_confidence": j.LocationConfidence,
		"remote_type":         j.RemoteType,
		"employment_type":     j.EmploymentType,
		"salary_min":          j.SalaryMin,
		"salary_max":          j.SalaryMax,
		"salary_currency":     j.SalaryCurrency,
		"apply_url":           j.ApplyURL,
		"posted_at":           j.PostedAt,
		"first_seen_at":       j.FirstSeenAt,
	}
}

func toDetail(j Job) map[string]any {
	detail := toSummary(j)
	detail["description"] = j.Description
	detail["source_url"] = j.SourceURL
	detail["status"] = j.Status
	return detail
}
