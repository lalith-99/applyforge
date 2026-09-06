package matching

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/candidateprofile"
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
	repo              *Repository
	candidateSkills   *candidateskills.Repository
	jobsRepo          *jobs.Repository
	requirementsSvc   *jobrequirements.Service
	preferencesRepo   *preferences.Repository
	profileRepo       *profile.Repository
	candidateProfiles *candidateprofile.Repository
}

// NewService builds a Service. candidateProfiles may be nil if Recommend
// (Phase G) isn't needed by the caller - Match works fine without it.
func NewService(repo *Repository, candidateSkillsRepo *candidateskills.Repository, jobsRepo *jobs.Repository, requirementsSvc *jobrequirements.Service, preferencesRepo *preferences.Repository, profileRepo *profile.Repository, candidateProfiles *candidateprofile.Repository) *Service {
	return &Service{
		repo:              repo,
		candidateSkills:   candidateSkillsRepo,
		jobsRepo:          jobsRepo,
		requirementsSvc:   requirementsSvc,
		preferencesRepo:   preferencesRepo,
		profileRepo:       profileRepo,
		candidateProfiles: candidateProfiles,
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

// RankedJob pairs a scored Result with the job it was computed for.
type RankedJob struct {
	Job    jobs.Job
	Result Result
}

// Recommend runs the multi-stage funnel (Phase G): hard filters -> semantic
// retrieval -> deterministic scoring. Returns up to limit ELIGIBLE jobs for
// userID, ranked by Result.TotalScore. Returns candidateprofile.ErrNotFound
// if the user has no generated (embedded) profile yet - callers should fall
// back to plain List()-based browsing in that case.
func (s *Service) Recommend(ctx context.Context, userID uuid.UUID, limit int) ([]RankedJob, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	embedding, err := s.candidateProfiles.GetLatestEmbedding(ctx, userID)
	if err != nil {
		return nil, err
	}

	prefs, err := s.preferencesRepo.Get(ctx, userID)
	if err != nil && err != preferences.ErrNotFound {
		return nil, err
	}

	filter := jobs.EmbeddingSearchFilter{}
	if prefs.Remote && !prefs.Hybrid && !prefs.Onsite {
		filter.RemoteType = "remote"
	}

	// Stage B (semantic retrieval) draws a generously-sized pool - stage C
	// (deterministic scoring, below) is what actually determines the final
	// ranking/cut, so the pool just needs to comfortably contain the true
	// top `limit` rather than being an exact candidate set itself.
	poolSize := int32(limit * 10) // #nosec G115 -- limit is clamped to <=100 above
	if poolSize < 200 {
		poolSize = 200
	}
	matches, err := s.jobsRepo.SearchByEmbedding(ctx, embedding, poolSize, filter)
	if err != nil {
		return nil, err
	}

	ranked := make([]RankedJob, 0, len(matches))
	for _, m := range matches {
		result, err := s.Match(ctx, m.Job.ID, userID)
		if err != nil {
			slog.Error("match failed during recommend", "job_id", m.Job.ID, "user_id", userID, "error", err)
			continue
		}
		if !result.Eligibility.Eligible {
			continue
		}
		ranked = append(ranked, RankedJob{Job: m.Job, Result: result})
	}

	sort.Slice(ranked, func(i, j int) bool { return ranked[i].Result.TotalScore > ranked[j].Result.TotalScore })
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked, nil
}
