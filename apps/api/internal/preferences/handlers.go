package preferences

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
)

// Handlers wires the preferences Repository to HTTP routes.
type Handlers struct {
	repo *Repository
}

// NewHandlers builds preferences Handlers.
func NewHandlers(repo *Repository) *Handlers {
	return &Handlers{repo: repo}
}

// Mount registers preferences routes onto r. Callers must apply
// auth.RequireAuth before mounting, since every route here is user-scoped.
func (h *Handlers) Mount(r chi.Router) {
	r.Get("/preferences", h.handleGet)
	r.Patch("/preferences", h.handleUpdate)
}

type preferencesRequest struct {
	Remote                              bool     `json:"remote"`
	Hybrid                              bool     `json:"hybrid"`
	Onsite                              bool     `json:"onsite"`
	PreferredLocations                  []string `json:"preferred_locations"`
	WillingnessToRelocate               bool     `json:"willingness_to_relocate"`
	EmploymentTypes                     []string `json:"employment_types"`
	MinimumSalary                       *int32   `json:"minimum_salary"`
	ExcludedCompanies                   []string `json:"excluded_companies"`
	ExcludedLocations                   []string `json:"excluded_locations"`
	ExcludedIndustries                  []string `json:"excluded_industries"`
	ClearanceConstraints                *string  `json:"clearance_constraints"`
	WorkAuthorization                   *string  `json:"work_authorization"`
	ImmigrationStatus                   *string  `json:"immigration_status"`
	RequiresH1BTransfer                 bool     `json:"requires_h1b_transfer"`
	RequiresNewH1BCapSponsorship        bool     `json:"requires_new_h1b_cap_sponsorship"`
	RequiresFutureEmploymentSponsorship bool     `json:"requires_future_employment_sponsorship"`
	GreenCardSupportPreferred           bool     `json:"green_card_support_preferred"`
	GreenCardSupportRequired            bool     `json:"green_card_support_required"`
	PermSupportPreferred                bool     `json:"perm_support_preferred"`
	ImmigrationSupportMinConfidence     *string  `json:"immigration_support_min_confidence"`
}

func toResponse(p Preferences) map[string]any {
	return map[string]any{
		"user_id":                                p.UserID,
		"remote":                                 p.Remote,
		"hybrid":                                 p.Hybrid,
		"onsite":                                 p.Onsite,
		"preferred_locations":                    p.PreferredLocations,
		"willingness_to_relocate":                p.WillingnessToRelocate,
		"employment_types":                       p.EmploymentTypes,
		"minimum_salary":                         p.MinimumSalary,
		"excluded_companies":                     p.ExcludedCompanies,
		"excluded_locations":                     p.ExcludedLocations,
		"excluded_industries":                    p.ExcludedIndustries,
		"clearance_constraints":                  p.ClearanceConstraints,
		"work_authorization":                     p.WorkAuthorization,
		"immigration_status":                     p.ImmigrationStatus,
		"requires_h1b_transfer":                  p.RequiresH1BTransfer,
		"requires_new_h1b_cap_sponsorship":       p.RequiresNewH1BCapSponsorship,
		"requires_future_employment_sponsorship": p.RequiresFutureEmploymentSponsorship,
		"green_card_support_preferred":           p.GreenCardSupportPreferred,
		"green_card_support_required":            p.GreenCardSupportRequired,
		"perm_support_preferred":                 p.PermSupportPreferred,
		"immigration_support_min_confidence":     p.ImmigrationSupportMinConfidence,
		"created_at":                             p.CreatedAt,
		"updated_at":                             p.UpdatedAt,
	}
}

func (h *Handlers) handleGet(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	p, err := h.repo.Get(r.Context(), u.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.WriteJSON(w, http.StatusOK, toResponse(Preferences{UserID: u.ID}))
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "could not load preferences")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toResponse(p))
}

func (h *Handlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req preferencesRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	p, err := h.repo.Upsert(r.Context(), u.ID, UpsertInput{
		Remote:                              req.Remote,
		Hybrid:                              req.Hybrid,
		Onsite:                              req.Onsite,
		PreferredLocations:                  req.PreferredLocations,
		WillingnessToRelocate:               req.WillingnessToRelocate,
		EmploymentTypes:                     req.EmploymentTypes,
		MinimumSalary:                       req.MinimumSalary,
		ExcludedCompanies:                   req.ExcludedCompanies,
		ExcludedLocations:                   req.ExcludedLocations,
		ExcludedIndustries:                  req.ExcludedIndustries,
		ClearanceConstraints:                req.ClearanceConstraints,
		WorkAuthorization:                   req.WorkAuthorization,
		ImmigrationStatus:                   req.ImmigrationStatus,
		RequiresH1BTransfer:                 req.RequiresH1BTransfer,
		RequiresNewH1BCapSponsorship:        req.RequiresNewH1BCapSponsorship,
		RequiresFutureEmploymentSponsorship: req.RequiresFutureEmploymentSponsorship,
		GreenCardSupportPreferred:           req.GreenCardSupportPreferred,
		GreenCardSupportRequired:            req.GreenCardSupportRequired,
		PermSupportPreferred:                req.PermSupportPreferred,
		ImmigrationSupportMinConfidence:     req.ImmigrationSupportMinConfidence,
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not update preferences")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toResponse(p))
}
