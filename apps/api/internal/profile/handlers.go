package profile

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/lalithlochan/applyforge/apps/api/internal/auth"
	"github.com/lalithlochan/applyforge/apps/api/internal/httpx"
)

// Handlers wires the profile Repository to HTTP routes.
type Handlers struct {
	repo *Repository
}

// NewHandlers builds profile Handlers.
func NewHandlers(repo *Repository) *Handlers {
	return &Handlers{repo: repo}
}

// Mount registers profile routes onto r. Callers must apply auth.RequireAuth
// (or equivalent) before mounting, since every route here is user-scoped.
func (h *Handlers) Mount(r chi.Router) {
	r.Get("/profile", h.handleGet)
	r.Patch("/profile", h.handleUpdate)
}

type profileRequest struct {
	FirstName                   *string  `json:"first_name"`
	LastName                    *string  `json:"last_name"`
	City                        *string  `json:"city"`
	State                       *string  `json:"state"`
	Country                     *string  `json:"country"`
	PrimaryTargetTitles         []string `json:"primary_target_titles"`
	AlternativeTargetTitles     []string `json:"alternative_target_titles"`
	Seniority                   *string  `json:"seniority"`
	YearsExperience             *int32   `json:"years_experience"`
	PreferredIndustries         []string `json:"preferred_industries"`
	PreferredTechnologies       []string `json:"preferred_technologies"`
	DesiredCompensationMin      *int32   `json:"desired_compensation_min"`
	DesiredCompensationMax      *int32   `json:"desired_compensation_max"`
	DesiredCompensationCurrency string   `json:"desired_compensation_currency"`
	CompleteOnboarding          bool     `json:"complete_onboarding"`
}

func toResponse(p Profile) map[string]any {
	return map[string]any{
		"user_id":                       p.UserID,
		"first_name":                    p.FirstName,
		"last_name":                     p.LastName,
		"city":                          p.City,
		"state":                         p.State,
		"country":                       p.Country,
		"primary_target_titles":         p.PrimaryTargetTitles,
		"alternative_target_titles":     p.AlternativeTargetTitles,
		"seniority":                     p.Seniority,
		"years_experience":              p.YearsExperience,
		"preferred_industries":          p.PreferredIndustries,
		"preferred_technologies":        p.PreferredTechnologies,
		"desired_compensation_min":      p.DesiredCompensationMin,
		"desired_compensation_max":      p.DesiredCompensationMax,
		"desired_compensation_currency": p.DesiredCompensationCurrency,
		"onboarding_completed_at":       p.OnboardingCompletedAt,
		"created_at":                    p.CreatedAt,
		"updated_at":                    p.UpdatedAt,
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
			httpx.WriteJSON(w, http.StatusOK, toResponse(Profile{UserID: u.ID, DesiredCompensationCurrency: "USD"}))
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "could not load profile")
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

	var req profileRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	p, err := h.repo.Upsert(r.Context(), u.ID, UpsertInput{
		FirstName:                   req.FirstName,
		LastName:                    req.LastName,
		City:                        req.City,
		State:                       req.State,
		Country:                     req.Country,
		PrimaryTargetTitles:         req.PrimaryTargetTitles,
		AlternativeTargetTitles:     req.AlternativeTargetTitles,
		Seniority:                   req.Seniority,
		YearsExperience:             req.YearsExperience,
		PreferredIndustries:         req.PreferredIndustries,
		PreferredTechnologies:       req.PreferredTechnologies,
		DesiredCompensationMin:      req.DesiredCompensationMin,
		DesiredCompensationMax:      req.DesiredCompensationMax,
		DesiredCompensationCurrency: req.DesiredCompensationCurrency,
		MarkOnboardingComplete:      req.CompleteOnboarding,
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not update profile")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toResponse(p))
}
