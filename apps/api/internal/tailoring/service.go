package tailoring

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/candidateskills"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobrequirements"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobs"
	"github.com/lalithlochan/applyforge/apps/api/internal/matching"
	"github.com/lalithlochan/applyforge/apps/api/internal/resume"
)

// Service orchestrates a full tailoring run: gathering candidate/job data,
// calling the AI worker, computing before/after Resume Alignment scores, and
// persisting the run + suggestions.
type Service struct {
	repo            *Repository
	resumes         *resume.Repository
	candidateSkills *candidateskills.Repository
	jobsRepo        *jobs.Repository
	requirementsSvc *jobrequirements.Service
	matchingRepo    *matching.Repository
	aiClient        *aiclient.Client
}

// NewService builds a Service.
func NewService(repo *Repository, resumes *resume.Repository, candidateSkillsRepo *candidateskills.Repository, jobsRepo *jobs.Repository, requirementsSvc *jobrequirements.Service, matchingRepo *matching.Repository, aiClient *aiclient.Client) *Service {
	return &Service{
		repo:            repo,
		resumes:         resumes,
		candidateSkills: candidateSkillsRepo,
		jobsRepo:        jobsRepo,
		requirementsSvc: requirementsSvc,
		matchingRepo:    matchingRepo,
		aiClient:        aiClient,
	}
}

// Tailor runs a full tailoring pass for a user's resume against a job.
func (s *Service) Tailor(ctx context.Context, userID, jobID, resumeID uuid.UUID, mode string) (Run, []Suggestion, error) {
	job, err := s.jobsRepo.GetByID(ctx, jobID)
	if err != nil {
		return Run{}, nil, err
	}
	reqs, err := s.requirementsSvc.GetOrParse(ctx, job.ID, job.Title, job.Description, job.ContentHash)
	if err != nil {
		return Run{}, nil, err
	}

	res, err := s.resumes.Get(ctx, resumeID, userID)
	if err != nil {
		return Run{}, nil, err
	}
	experiences, err := s.resumes.ListExperiences(ctx, resumeID)
	if err != nil {
		return Run{}, nil, err
	}

	skills, err := s.candidateSkills.ListForUser(ctx, userID)
	if err != nil {
		return Run{}, nil, err
	}

	skillSet := make(map[string]bool, len(skills))
	masterSkills := make([]string, 0, len(skills))
	skillKeys := make([]string, 0, len(skills))
	for _, sk := range skills {
		key := strings.ToLower(sk.NormalizedName)
		skillSet[key] = true
		masterSkills = append(masterSkills, sk.DisplayName)
		skillKeys = append(skillKeys, key)
	}

	transferable, err := s.matchingRepo.ListTransferableFromSkills(ctx, skillKeys)
	if err != nil {
		return Run{}, nil, err
	}

	requiredNames := skillRequirementNames(reqs.RequiredSkills)
	preferredNames := skillRequirementNames(reqs.PreferredSkills)

	alignmentBefore := ComputeAlignment(skillSet, requiredNames, preferredNames, reqs.Responsibilities)

	run, err := s.repo.CreateRun(ctx, userID, jobID, resumeID, mode, int32(alignmentBefore))
	if err != nil {
		return Run{}, nil, err
	}

	var masterSummary *string
	var parsedProfile struct {
		Summary *string `json:"summary"`
	}
	if len(res.ParsedProfile) > 0 {
		if err := json.Unmarshal(res.ParsedProfile, &parsedProfile); err == nil {
			masterSummary = parsedProfile.Summary
		}
	}

	aiReq := aiclient.TailoringRequest{
		Mode:                mode,
		JobTitle:            job.Title,
		MasterSkills:        masterSkills,
		MasterSummary:       masterSummary,
		Experiences:         toAIExperiences(experiences),
		RequiredSkills:      requiredNames,
		PreferredSkills:     preferredNames,
		Responsibilities:    reqs.Responsibilities,
		TransferableMatches: toAITransferable(transferable),
	}

	aiResp, err := s.aiClient.SuggestTailoring(ctx, aiReq)
	if err != nil {
		_ = s.repo.FailRun(ctx, run.ID)
		return Run{}, nil, err
	}

	var suggestions []Suggestion
	if aiResp.SummarySuggestion != nil {
		created, err := s.repo.AddSuggestion(ctx, run.ID, fromAISuggestion(*aiResp.SummarySuggestion))
		if err == nil {
			suggestions = append(suggestions, created)
		}
	}
	for _, sg := range aiResp.SkillSuggestions {
		created, err := s.repo.AddSuggestion(ctx, run.ID, fromAISuggestion(sg))
		if err == nil {
			suggestions = append(suggestions, created)
		}
	}
	for _, sg := range aiResp.ExperienceSuggestions {
		created, err := s.repo.AddSuggestion(ctx, run.ID, fromAISuggestion(sg))
		if err == nil {
			suggestions = append(suggestions, created)
		}
	}

	// Projected "after" skill set credits AI-suggested skills too, since the
	// alignment score after tailoring reflects what approving suggestions
	// would achieve — never presented as already-verified current capability.
	afterSkillSet := make(map[string]bool, len(skillSet)+len(aiResp.SkillSuggestions))
	for k, v := range skillSet {
		afterSkillSet[k] = v
	}
	for _, sg := range aiResp.SkillSuggestions {
		for _, added := range sg.SkillsAdded {
			afterSkillSet[strings.ToLower(added)] = true
		}
	}
	alignmentAfter := ComputeAlignment(afterSkillSet, requiredNames, preferredNames, reqs.Responsibilities)

	summaryJSON, _ := json.Marshal(aiResp.SummarySuggestion)
	coverageJSON, _ := json.Marshal(map[string]float64{
		"before": aiResp.KeywordCoverageBefore,
		"after":  aiResp.KeywordCoverageAfter,
	})

	completedRun, err := s.repo.CompleteRun(ctx, run.ID, summaryJSON, coverageJSON, int32(alignmentAfter))
	if err != nil {
		return Run{}, nil, err
	}

	return completedRun, suggestions, nil
}

func skillRequirementNames(reqs []aiclient.SkillRequirement) []string {
	out := make([]string, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, r.NormalizedName)
	}
	return out
}

func toAIExperiences(experiences []resume.Experience) []aiclient.TailoringExperience {
	out := make([]aiclient.TailoringExperience, 0, len(experiences))
	for _, e := range experiences {
		out = append(out, aiclient.TailoringExperience{
			Company:        stringOrEmpty(e.Company),
			Title:          stringOrEmpty(e.Title),
			Bullets:        e.Bullets,
			DetectedSkills: e.DetectedSkills,
		})
	}
	return out
}

func toAITransferable(transfers []matching.TransferableSkill) []aiclient.TailoringTransferableMatch {
	out := make([]aiclient.TailoringTransferableMatch, 0, len(transfers))
	for _, t := range transfers {
		out = append(out, aiclient.TailoringTransferableMatch{
			SourceSkill:        t.SourceSkill,
			TargetSkill:        t.TargetSkill,
			Level:              t.Level,
			PrepClassification: t.PrepClassification,
		})
	}
	return out
}

func fromAISuggestion(s aiclient.TailoringSuggestion) Suggestion {
	return Suggestion{
		Section:               s.Section,
		OriginalText:          s.OriginalText,
		SuggestedText:         s.SuggestedText,
		RequirementsAddressed: s.RequirementsAddressed,
		SkillsAdded:           s.SkillsAdded,
		KeywordsAdded:         s.KeywordsAdded,
		Source:                s.Source,
		Reason:                s.Reason,
		Confidence:            s.Confidence,
		RiskLevel:             s.RiskLevel,
		UserStatus:            StatusPending,
	}
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
