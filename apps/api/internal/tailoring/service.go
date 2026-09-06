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

// Tailor runs a full tailoring pass for a user's resume against a job,
// synchronously. Kept for tests/manual use; the HTTP handler uses the async
// CreateQueuedRun + ProcessRun split instead (Phase J), since generation
// (plus Phase K's critique/revision pass) can take a while and the user
// explicitly doesn't need it to block the request.
func (s *Service) Tailor(ctx context.Context, userID, jobID, resumeID uuid.UUID, mode string) (Run, []Suggestion, error) {
	run, err := s.CreateQueuedRun(ctx, userID, jobID, resumeID, mode)
	if err != nil {
		return Run{}, nil, err
	}
	if err := s.ProcessRun(ctx, run.ID); err != nil {
		return Run{}, nil, err
	}
	completed, err := s.repo.GetRun(ctx, run.ID)
	if err != nil {
		return Run{}, nil, err
	}
	suggestions, err := s.repo.ListSuggestions(ctx, run.ID)
	if err != nil {
		return Run{}, nil, err
	}
	return completed, suggestions, nil
}

// CreateQueuedRun gathers just enough data to record the before-tailoring
// alignment score and create a PENDING run - no AI generation call happens
// here, so this is fast and safe to call synchronously from an HTTP handler.
func (s *Service) CreateQueuedRun(ctx context.Context, userID, jobID, resumeID uuid.UUID, mode string) (Run, error) {
	job, err := s.jobsRepo.GetByID(ctx, jobID)
	if err != nil {
		return Run{}, err
	}
	reqs, err := s.requirementsSvc.GetOrParse(ctx, job.ID, job.Title, job.Description, job.ContentHash)
	if err != nil {
		return Run{}, err
	}

	skills, err := s.candidateSkills.ListForUser(ctx, userID)
	if err != nil {
		return Run{}, err
	}
	skillSet := make(map[string]bool, len(skills))
	for _, sk := range skills {
		skillSet[strings.ToLower(sk.NormalizedName)] = true
	}

	requiredNames := skillRequirementNames(reqs.RequiredSkills)
	preferredNames := skillRequirementNames(reqs.PreferredSkills)
	alignmentBefore := ComputeAlignment(skillSet, requiredNames, preferredNames, reqs.Responsibilities)

	return s.repo.CreateRun(ctx, userID, jobID, resumeID, mode, int32(alignmentBefore))
}

// maxRevisions bounds Phase K's critique-driven regeneration loop - one
// revision pass is enough to meaningfully improve a flagged first draft
// without unbounded AI cost/latency if the critic keeps objecting.
const maxRevisions = 1

// ProcessRun runs the async multi-pass pipeline for an already-created
// PENDING run (Phase J: generation; Phase K: AI critique + bounded
// revision), called by a background worker. Advances Run.Status through
// WRITING -> EVALUATING -> (REVISING -> WRITING -> EVALUATING once, if the
// critic recommends it) -> COMPLETED, or FAILED on error.
func (s *Service) ProcessRun(ctx context.Context, runID uuid.UUID) error {
	run, err := s.repo.GetRun(ctx, runID)
	if err != nil {
		return err
	}

	job, err := s.jobsRepo.GetByID(ctx, run.JobID)
	if err != nil {
		_ = s.repo.FailRun(ctx, runID)
		return err
	}
	reqs, err := s.requirementsSvc.GetOrParse(ctx, job.ID, job.Title, job.Description, job.ContentHash)
	if err != nil {
		_ = s.repo.FailRun(ctx, runID)
		return err
	}
	res, err := s.resumes.Get(ctx, run.ResumeID, run.UserID)
	if err != nil {
		_ = s.repo.FailRun(ctx, runID)
		return err
	}
	experiences, err := s.resumes.ListExperiences(ctx, run.ResumeID)
	if err != nil {
		_ = s.repo.FailRun(ctx, runID)
		return err
	}
	skills, err := s.candidateSkills.ListForUser(ctx, run.UserID)
	if err != nil {
		_ = s.repo.FailRun(ctx, runID)
		return err
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
		_ = s.repo.FailRun(ctx, runID)
		return err
	}

	requiredNames := skillRequirementNames(reqs.RequiredSkills)
	preferredNames := skillRequirementNames(reqs.PreferredSkills)

	var masterSummary *string
	var parsedProfile struct {
		Summary *string `json:"summary"`
	}
	if len(res.ParsedProfile) > 0 {
		if err := json.Unmarshal(res.ParsedProfile, &parsedProfile); err == nil {
			masterSummary = parsedProfile.Summary
		}
	}

	baseReq := aiclient.TailoringRequest{
		Mode:                run.Mode,
		JobTitle:            job.Title,
		MasterSkills:        masterSkills,
		MasterSummary:       masterSummary,
		Experiences:         toAIExperiences(experiences),
		RequiredSkills:      requiredNames,
		PreferredSkills:     preferredNames,
		Responsibilities:    reqs.Responsibilities,
		TransferableMatches: toAITransferable(transferable),
	}

	if err := s.repo.UpdateStatus(ctx, runID, RunStatusWriting); err != nil {
		return err
	}
	aiResp, err := s.aiClient.SuggestTailoring(ctx, baseReq)
	if err != nil {
		_ = s.repo.FailRun(ctx, runID)
		return err
	}

	if err := s.repo.UpdateStatus(ctx, runID, RunStatusEvaluating); err != nil {
		return err
	}
	critic, criticErr := s.aiClient.Critique(ctx, buildCritiqueRequest(job.Title, masterSummary, masterSkills, requiredNames, preferredNames, aiResp))

	revisionCount := int32(0)
	if criticErr == nil && critic.RecommendRegeneration && revisionCount < maxRevisions {
		if err := s.repo.UpdateStatus(ctx, runID, RunStatusRevising); err != nil {
			return err
		}
		revisedReq := baseReq
		revisedReq.Responsibilities = append(append([]string{}, reqs.Responsibilities...),
			"CRITIC FEEDBACK FROM PREVIOUS DRAFT (address this): "+critic.Feedback)

		if err := s.repo.UpdateStatus(ctx, runID, RunStatusWriting); err != nil {
			return err
		}
		if revisedResp, revErr := s.aiClient.SuggestTailoring(ctx, revisedReq); revErr == nil {
			aiResp = revisedResp
			revisionCount = 1

			if err := s.repo.UpdateStatus(ctx, runID, RunStatusEvaluating); err != nil {
				return err
			}
			if reCritic, reErr := s.aiClient.Critique(ctx, buildCritiqueRequest(job.Title, masterSummary, masterSkills, requiredNames, preferredNames, aiResp)); reErr == nil {
				critic = reCritic
			}
		}
	}

	if criticErr == nil {
		if criticJSON, err := json.Marshal(critic); err == nil {
			_ = s.repo.SetCritic(ctx, runID, criticJSON, revisionCount)
		}
	}

	var suggestions []Suggestion
	if aiResp.SummarySuggestion != nil {
		if created, err := s.repo.AddSuggestion(ctx, runID, fromAISuggestion(*aiResp.SummarySuggestion)); err == nil {
			suggestions = append(suggestions, created)
		}
	}
	for _, sg := range aiResp.SkillSuggestions {
		if created, err := s.repo.AddSuggestion(ctx, runID, fromAISuggestion(sg)); err == nil {
			suggestions = append(suggestions, created)
		}
	}
	for _, sg := range aiResp.ExperienceSuggestions {
		if created, err := s.repo.AddSuggestion(ctx, runID, fromAISuggestion(sg)); err == nil {
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

	_, err = s.repo.CompleteRun(ctx, runID, summaryJSON, coverageJSON, int32(alignmentAfter))
	return err
}

func buildCritiqueRequest(jobTitle string, masterSummary *string, masterSkills, requiredNames, preferredNames []string, aiResp aiclient.TailoringResponse) aiclient.CritiqueRequest {
	all := make([]aiclient.CritiqueSuggestion, 0, len(aiResp.SkillSuggestions)+len(aiResp.ExperienceSuggestions)+1)
	if aiResp.SummarySuggestion != nil {
		all = append(all, toCritiqueSuggestion(*aiResp.SummarySuggestion))
	}
	for _, sg := range aiResp.SkillSuggestions {
		all = append(all, toCritiqueSuggestion(sg))
	}
	for _, sg := range aiResp.ExperienceSuggestions {
		all = append(all, toCritiqueSuggestion(sg))
	}
	return aiclient.CritiqueRequest{
		JobTitle:            jobTitle,
		MasterResumeSummary: masterSummary,
		MasterSkills:        masterSkills,
		RequiredSkills:      requiredNames,
		PreferredSkills:     preferredNames,
		Suggestions:         all,
	}
}

func toCritiqueSuggestion(s aiclient.TailoringSuggestion) aiclient.CritiqueSuggestion {
	return aiclient.CritiqueSuggestion{
		Section:       s.Section,
		SuggestedText: s.SuggestedText,
		SkillsAdded:   s.SkillsAdded,
		Source:        s.Source,
		RiskLevel:     s.RiskLevel,
	}
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
