package learning

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/candidateskills"
	"github.com/lalithlochan/applyforge/apps/api/internal/matching"
)

// Service orchestrates Quick Prep, Defend This Bullet, learning plans, and
// Make Me Qualified, combining cached AI-worker content with the
// deterministic matching engine.
type Service struct {
	repo            *Repository
	aiClient        *aiclient.Client
	candidateSkills *candidateskills.Repository
	matchingRepo    *matching.Repository
	matchingSvc     *matching.Service
}

// NewService builds a Service.
func NewService(repo *Repository, aiClient *aiclient.Client, candidateSkillsRepo *candidateskills.Repository, matchingRepo *matching.Repository, matchingSvc *matching.Service) *Service {
	return &Service{
		repo:            repo,
		aiClient:        aiClient,
		candidateSkills: candidateSkillsRepo,
		matchingRepo:    matchingRepo,
		matchingSvc:     matchingSvc,
	}
}

// QuickPrep returns (generating + caching on first request) a Quick Prep
// module for a skill, personalized with the caller's transferable-skill
// sources at request time (transfer personalization is never cached, since
// the underlying quick_prep_modules cache is shared across all users).
func (s *Service) QuickPrep(ctx context.Context, userID uuid.UUID, skill string) (aiclient.QuickPrepModule, error) {
	key := normalizedKey(skill)

	module, err := s.repo.GetCachedQuickPrep(ctx, key)
	if errors.Is(err, ErrNotFound) {
		module, err = s.aiClient.GenerateQuickPrep(ctx, skill, nil)
		if err != nil {
			return aiclient.QuickPrepModule{}, err
		}
		module.TransferableFrom = nil // never cache personalized data
		if saveErr := s.repo.SaveQuickPrep(ctx, key, module); saveErr != nil {
			return aiclient.QuickPrepModule{}, saveErr
		}
	} else if err != nil {
		return aiclient.QuickPrepModule{}, err
	}

	module.Skill = skill
	module.TransferableFrom = s.personalizedTransfers(ctx, userID, skill)
	return module, nil
}

func (s *Service) personalizedTransfers(ctx context.Context, userID uuid.UUID, skill string) []string {
	if userID == uuid.Nil {
		return nil
	}
	skills, err := s.candidateSkills.ListForUser(ctx, userID)
	if err != nil || len(skills) == 0 {
		return nil
	}
	keys := make([]string, 0, len(skills))
	for _, sk := range skills {
		keys = append(keys, strings.ToLower(sk.NormalizedName))
	}
	transfers, err := s.matchingRepo.ListTransferableFromSkills(ctx, keys)
	if err != nil {
		return nil
	}
	var sources []string
	for _, t := range transfers {
		if strings.EqualFold(t.TargetSkill, skill) {
			sources = append(sources, t.SourceSkill)
		}
	}
	return sources
}

// DefendBullet returns likely interview questions for a resume bullet. Not
// cached, since bullet text and skill combinations vary per suggestion.
func (s *Service) DefendBullet(ctx context.Context, bulletText string, skills []string) (aiclient.DefendBulletResponse, error) {
	return s.aiClient.DefendBullet(ctx, bulletText, skills)
}

// LearningPlan generates (and caches) a "Prepare for This Job" plan covering
// a user's missing skills for a job.
func (s *Service) LearningPlan(ctx context.Context, userID, jobID uuid.UUID) (PlanResult, error) {
	match, err := s.matchingSvc.Match(ctx, jobID, userID)
	if err != nil {
		return PlanResult{}, err
	}

	missing := append(append([]string{}, match.MissingRequiredSkills...), match.MissingPreferredSkills...)

	generated, err := s.aiClient.GenerateLearningPlan(ctx, "", missing, match.CurrentProfileMatch, match.TargetProfileMatch)
	if err != nil {
		return PlanResult{}, err
	}

	return s.repo.SaveLearningPlan(ctx, userID, jobID, generated)
}

// QualifiedResult is the aggregated "Make Me Qualified" response (§33).
type QualifiedResult struct {
	CurrentProfileMatch int
	TargetProfileMatch  int
	HighValueGaps       []string
	LowValueGaps        []string
	RecommendedSkills   []string
	InterviewReadiness  int
	ReadinessComponents ReadinessComponents
	LearningPlan        PlanResult
}

// MakeMeQualified analyzes a job and returns strengths, gaps, and a learning
// plan in one aggregated call.
func (s *Service) MakeMeQualified(ctx context.Context, userID, jobID uuid.UUID) (QualifiedResult, error) {
	match, err := s.matchingSvc.Match(ctx, jobID, userID)
	if err != nil {
		return QualifiedResult{}, err
	}

	// High-value gaps: required skills missing or only transferable.
	// Low-value gaps: missing preferred skills (nice-to-have, lower priority).
	highValue := append(append([]string{}, match.MissingRequiredSkills...), transferableTargets(match.TransferableSkills)...)
	lowValue := match.MissingPreferredSkills

	plan, err := s.LearningPlan(ctx, userID, jobID)
	if err != nil {
		return QualifiedResult{}, err
	}

	readinessScore, readinessComponents := InterviewReadiness(match)

	return QualifiedResult{
		CurrentProfileMatch: match.CurrentProfileMatch,
		TargetProfileMatch:  match.TargetProfileMatch,
		HighValueGaps:       highValue,
		LowValueGaps:        lowValue,
		RecommendedSkills:   match.SuggestedTargetAdditions,
		InterviewReadiness:  readinessScore,
		ReadinessComponents: readinessComponents,
		LearningPlan:        plan,
	}, nil
}

func transferableTargets(transfers []matching.TransferableMatch) []string {
	out := make([]string, 0, len(transfers))
	for _, t := range transfers {
		out = append(out, t.TargetSkill)
	}
	return out
}
