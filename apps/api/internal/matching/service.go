package matching

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/candidateskills"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobrequirements"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobs"
	"github.com/lalithlochan/applyforge/apps/api/internal/preferences"
	"github.com/lalithlochan/applyforge/apps/api/internal/profile"
)

// Service orchestrates fetching candidate/job data, scoring, and caching the
// result. It's the single place that stitches together the otherwise
// decoupled candidateskills/jobs/jobrequirements/preferences packages.
type Service struct {
	repo            *Repository
	candidateSkills *candidateskills.Repository
	jobsRepo        *jobs.Repository
	requirementsSvc *jobrequirements.Service
	preferencesRepo *preferences.Repository
	profileRepo     *profile.Repository
}

// NewService builds a Service.
func NewService(repo *Repository, candidateSkillsRepo *candidateskills.Repository, jobsRepo *jobs.Repository, requirementsSvc *jobrequirements.Service, preferencesRepo *preferences.Repository, profileRepo *profile.Repository) *Service {
	return &Service{
		repo:            repo,
		candidateSkills: candidateSkillsRepo,
		jobsRepo:        jobsRepo,
		requirementsSvc: requirementsSvc,
		preferencesRepo: preferencesRepo,
		profileRepo:     profileRepo,
	}
}

// Match computes (and caches) the deterministic match Result for a user against a job.
func (s *Service) Match(ctx context.Context, jobID, userID uuid.UUID) (Result, error) {
	job, err := s.jobsRepo.GetByID(ctx, jobID)
	if err != nil {
		return Result{}, err
	}

	reqs, err := s.requirementsSvc.GetOrParse(ctx, job.ID, job.Title, job.Description, job.ContentHash)
	if err != nil {
		return Result{}, err
	}

	skills, err := s.candidateSkills.ListForUser(ctx, userID)
	if err != nil {
		return Result{}, err
	}

	prefs, err := s.preferencesRepo.Get(ctx, userID)
	if err != nil && err != preferences.ErrNotFound {
		return Result{}, err
	}

	candidateProfile, err := s.profileRepo.Get(ctx, userID)
	if err != nil && err != profile.ErrNotFound {
		return Result{}, err
	}

	candidateSkillSet := make(map[string]bool, len(skills))
	candidateTargetSkillSet := make(map[string]bool)
	skillKeys := make([]string, 0, len(skills))
	for _, sk := range skills {
		key := strings.ToLower(sk.NormalizedName)
		skillKeys = append(skillKeys, key)
		if sk.Status == candidateskills.StatusUserApproved || sk.Status == candidateskills.StatusTargetSkill {
			candidateTargetSkillSet[key] = true
		} else {
			candidateSkillSet[key] = true
		}
	}

	transferable, err := s.repo.ListTransferableFromSkills(ctx, skillKeys)
	if err != nil {
		return Result{}, err
	}

	input := Input{
		CandidateSkills:          candidateSkillSet,
		CandidateTargetSkills:    candidateTargetSkillSet,
		TransferableFromSkills:   transferable,
		CandidateSeniority:       stringOrEmpty(candidateProfile.Seniority),
		PreferredRemote:          prefs.Remote,
		PreferredHybrid:          prefs.Hybrid,
		PreferredOnsite:          prefs.Onsite,
		PreferredEmploymentTypes: prefs.EmploymentTypes,
		ExcludedCompanies:        prefs.ExcludedCompanies,
		ExcludedLocations:        prefs.ExcludedLocations,
		CompanyName:              job.CompanyName,
		LocationText:             stringOrEmpty(job.LocationText),
		RemoteType:               stringOrEmpty(job.RemoteType),
		EmploymentType:           stringOrEmpty(job.EmploymentType),
		JobSeniority:             stringOrEmpty(reqs.Seniority),
		RequiredSkills:           toSkillRequirements(reqs.RequiredSkills),
		PreferredSkills:          toSkillRequirements(reqs.PreferredSkills),
		Responsibilities:         reqs.Responsibilities,
		HasEducationReqs:         len(reqs.EducationRequirements) > 0,
		HasCertReqs:              len(reqs.Certifications) > 0,
		PostedAt:                 job.PostedAt,
		FirstSeenAt:              job.FirstSeenAt,
	}

	result := Score(input)
	if err := s.repo.Save(ctx, jobID, userID, result); err != nil {
		return Result{}, err
	}
	return result, nil
}

func toSkillRequirements(reqs []aiclient.SkillRequirement) []SkillRequirement {
	out := make([]SkillRequirement, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, SkillRequirement{NormalizedName: strings.ToLower(r.NormalizedName), Importance: r.Importance})
	}
	return out
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
