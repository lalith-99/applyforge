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

	res, err := s.resumes.Get(ctx, resumeID, userID)
	if err != nil {
		return Run{}, err
	}
	experiences, err := s.resumes.ListExperiences(ctx, resumeID)
	if err != nil {
		return Run{}, err
	}
	skills, err := s.candidateSkills.ListForUser(ctx, userID)
	if err != nil {
		return Run{}, err
	}
	skillSet, _, _ := masterResumeSkillInventory(res, experiences, skills)

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

	skillSet, masterSkills, masterSummary := masterResumeSkillInventory(res, experiences, skills)
	skillKeys := make([]string, 0, len(skillSet))
	for key := range skillSet {
		skillKeys = append(skillKeys, key)
	}

	transferable, err := s.matchingRepo.ListTransferableFromSkills(ctx, skillKeys)
	if err != nil {
		_ = s.repo.FailRun(ctx, runID)
		return err
	}

	requiredNames := skillRequirementNames(reqs.RequiredSkills)
	preferredNames := skillRequirementNames(reqs.PreferredSkills)

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
	sanitizeKnownSkillSuggestions(&aiResp, skillSet)

	if err := s.repo.UpdateStatus(ctx, runID, RunStatusEvaluating); err != nil {
		return err
	}
	critic, criticErr := s.aiClient.Critique(ctx, buildCritiqueRequest(job.Title, masterSummary, masterSkills, requiredNames, preferredNames, reqs.Responsibilities, aiResp))

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
			sanitizeKnownSkillSuggestions(&revisedResp, skillSet)
			aiResp = revisedResp
			revisionCount = 1

			if err := s.repo.UpdateStatus(ctx, runID, RunStatusEvaluating); err != nil {
				return err
			}
			if reCritic, reErr := s.aiClient.Critique(ctx, buildCritiqueRequest(job.Title, masterSummary, masterSkills, requiredNames, preferredNames, reqs.Responsibilities, aiResp)); reErr == nil {
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

// masterResumeSkillInventory derives the skills actually present on the
// selected resume. This intentionally does not treat user-global target/AI
// skills as if they were already written on this particular resume.
func masterResumeSkillInventory(res resume.Resume, experiences []resume.Experience, fallback []candidateskills.Skill) (map[string]bool, []string, *string) {
	skillSet := map[string]bool{}
	masterSkills := []string{}
	seenDisplay := map[string]bool{}
	add := func(raw string) {
		display := strings.TrimSpace(raw)
		key := tailoringSkillKey(display)
		if key == "" || skillSet[key] {
			return
		}
		skillSet[key] = true
		if !seenDisplay[key] {
			masterSkills = append(masterSkills, display)
			seenDisplay[key] = true
		}
	}

	var masterSummary *string
	if len(res.ParsedProfile) > 0 {
		var parsed aiclient.ResumeProfile
		if err := json.Unmarshal(res.ParsedProfile, &parsed); err == nil {
			masterSummary = parsed.Summary
			for _, skill := range parsed.Skills {
				add(skill)
			}
		}
	}
	for _, exp := range experiences {
		for _, skill := range exp.DetectedSkills {
			add(skill)
		}
		for _, skill := range exp.Technologies {
			add(skill)
		}
	}

	// Older/partially parsed resumes may not have structured skill arrays.
	// Fall back only to skills whose provenance is the master resume, rather
	// than user-approved targeting suggestions from unrelated jobs.
	if len(skillSet) == 0 {
		for _, skill := range fallback {
			if skill.Source == candidateskills.SourceMasterResume {
				add(skill.DisplayName)
			}
		}
	}
	return skillSet, masterSkills, masterSummary
}

func tailoringSkillKey(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

// sanitizeKnownSkillSuggestions is a hard business-rule boundary around LLM
// output. The model may rewrite or emphasize a verified skill, but it must
// never label a skill already present on the selected resume as a new
// AI-suggested/learn-first addition.
func sanitizeKnownSkillSuggestions(resp *aiclient.TailoringResponse, known map[string]bool) {
	filteredSkills := make([]aiclient.TailoringSuggestion, 0, len(resp.SkillSuggestions))
	for _, suggestion := range resp.SkillSuggestions {
		if suggestionTouchesKnownSkill(suggestion, known) {
			continue
		}
		filteredSkills = append(filteredSkills, suggestion)
	}
	resp.SkillSuggestions = filteredSkills

	filteredExperiences := make([]aiclient.TailoringSuggestion, 0, len(resp.ExperienceSuggestions))
	for _, suggestion := range resp.ExperienceSuggestions {
		if suggestion.Source == "AI_SUGGESTED" && suggestionTouchesKnownSkill(suggestion, known) {
			continue
		}
		filteredExperiences = append(filteredExperiences, suggestion)
	}
	resp.ExperienceSuggestions = filteredExperiences

	if resp.SummarySuggestion != nil {
		resp.SummarySuggestion.SkillsAdded = unknownSkills(resp.SummarySuggestion.SkillsAdded, known)
		resp.SummarySuggestion.KeywordsAdded = unknownSkills(resp.SummarySuggestion.KeywordsAdded, known)
	}
}

func suggestionTouchesKnownSkill(suggestion aiclient.TailoringSuggestion, known map[string]bool) bool {
	for _, skill := range suggestion.SkillsAdded {
		if known[tailoringSkillKey(skill)] {
			return true
		}
	}
	return false
}

func unknownSkills(values []string, known map[string]bool) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if !known[tailoringSkillKey(value)] {
			out = append(out, value)
		}
	}
	return out
}

func buildCritiqueRequest(jobTitle string, masterSummary *string, masterSkills, requiredNames, preferredNames, responsibilities []string, aiResp aiclient.TailoringResponse) aiclient.CritiqueRequest {
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
		Responsibilities:    responsibilities,
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
