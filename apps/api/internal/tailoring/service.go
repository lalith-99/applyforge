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
// CreateQueuedRun + ProcessRun split instead, since generation and critique
// can take a while and the user does not need them to block the request.
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

// ProcessRun generates exactly one tailoring draft, then runs the critic as
// an advisory scorer. Critic feedback is persisted for ATS/readability scores,
// but it must never replace the user's first complete AI draft automatically.
// This keeps the user-facing suggestions stable and prevents a second model
// call from collapsing a strong multi-bullet draft into a weaker one.
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
		JobDescription:      job.Description,
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
	sanitizeTailoringSuggestions(&aiResp, skillSet, targetSkillKeys(reqs.RequiredSkills, reqs.PreferredSkills))

	if err := s.repo.UpdateStatus(ctx, runID, RunStatusEvaluating); err != nil {
		return err
	}
	critic, criticErr := s.aiClient.Critique(ctx, buildCritiqueRequest(job.Title, masterSummary, masterSkills, requiredNames, preferredNames, reqs.Responsibilities, aiResp))

	revisionCount := int32(0)

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
	// Show experience rewrites/new bullets before companion skill cards so
	// users evaluate evidence-bearing prose before approving a keyword.
	for _, sg := range aiResp.ExperienceSuggestions {
		if created, err := s.repo.AddSuggestion(ctx, runID, fromAISuggestion(sg)); err == nil {
			suggestions = append(suggestions, created)
		}
	}
	for _, sg := range aiResp.SkillSuggestions {
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
			afterSkillSet[tailoringSkillKey(added)] = true
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
	key := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
	switch key {
	case "react.js", "reactjs":
		return "react"
	case "node.js", "node js":
		return "nodejs"
	case "golang", "go (golang)":
		return "go"
	case "postgres":
		return "postgresql"
	case "amazon web services":
		return "aws"
	case "apache kafka":
		return "kafka"
	case "argo cd":
		return "argocd"
	}
	return key
}

func targetSkillKeys(required, preferred []aiclient.SkillRequirement) map[string]bool {
	allowed := map[string]bool{}
	for _, req := range append(append([]aiclient.SkillRequirement{}, required...), preferred...) {
		if len(req.Alternatives) > 0 {
			for _, alternative := range req.Alternatives {
				allowed[tailoringSkillKey(alternative)] = true
			}
			continue
		}
		if key := tailoringSkillKey(req.NormalizedName); key != "" {
			allowed[key] = true
		}
	}
	return allowed
}

// sanitizeKnownSkillSuggestions is retained for focused unit tests and legacy
// callers. Production tailoring additionally constrains standalone skill cards
// to actual extracted target-job skills via sanitizeTailoringSuggestions.
func sanitizeKnownSkillSuggestions(resp *aiclient.TailoringResponse, known map[string]bool) {
	sanitizeTailoringSuggestions(resp, known, nil)
}

func sanitizeTailoringSuggestions(resp *aiclient.TailoringResponse, known, allowed map[string]bool) {
	filteredSkills := make([]aiclient.TailoringSuggestion, 0, len(resp.SkillSuggestions))
	for _, suggestion := range resp.SkillSuggestions {
		suggestion.SkillsAdded = unknownSkills(suggestion.SkillsAdded, known)
		suggestion.KeywordsAdded = unknownSkills(suggestion.KeywordsAdded, known)
		suggestion.SkillCategories = filterSkillCategories(suggestion.SkillCategories, suggestion.SkillsAdded)
		if len(suggestion.SkillsAdded) == 0 {
			continue
		}
		if len(allowed) > 0 {
			keptSkills := make([]string, 0, len(suggestion.SkillsAdded))
			for _, skill := range suggestion.SkillsAdded {
				if allowed[tailoringSkillKey(skill)] {
					keptSkills = append(keptSkills, skill)
				}
			}
			if len(keptSkills) == 0 {
				continue
			}
			suggestion.SkillsAdded = keptSkills
			suggestion.KeywordsAdded = allowedSkills(suggestion.KeywordsAdded, allowed)
			suggestion.SkillCategories = filterSkillCategories(suggestion.SkillCategories, keptSkills)
		}
		filteredSkills = append(filteredSkills, suggestion)
	}
	resp.SkillSuggestions = filteredSkills

	// Experience rewrites/new bullets are valuable resume content and must not
	// be discarded merely because model metadata also names known technology.
	for i := range resp.ExperienceSuggestions {
		suggestion := &resp.ExperienceSuggestions[i]
		suggestion.SkillsAdded = unknownSkills(suggestion.SkillsAdded, known)
		suggestion.KeywordsAdded = unknownSkills(suggestion.KeywordsAdded, known)
		suggestion.SkillCategories = filterSkillCategories(suggestion.SkillCategories, suggestion.SkillsAdded)
		if suggestion.Source == "AI_SUGGESTED" && len(suggestion.SkillsAdded) == 0 && suggestion.Operation != OperationAdd {
			suggestion.Source = "MASTER_RESUME"
			suggestion.RiskLevel = "LOW"
		}
	}

	if resp.SummarySuggestion != nil {
		resp.SummarySuggestion.SkillsAdded = unknownSkills(resp.SummarySuggestion.SkillsAdded, known)
		resp.SummarySuggestion.KeywordsAdded = unknownSkills(resp.SummarySuggestion.KeywordsAdded, known)
		resp.SummarySuggestion.SkillCategories = filterSkillCategories(
			resp.SummarySuggestion.SkillCategories,
			resp.SummarySuggestion.SkillsAdded,
		)
		if resp.SummarySuggestion.Source == "AI_SUGGESTED" &&
			len(resp.SummarySuggestion.SkillsAdded) == 0 {
			resp.SummarySuggestion.Source = "MASTER_RESUME"
			resp.SummarySuggestion.RiskLevel = "LOW"
		}
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

func allowedSkills(values []string, allowed map[string]bool) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if allowed[tailoringSkillKey(value)] {
			out = append(out, value)
		}
	}
	return out
}

func filterSkillCategories(categories map[string]string, skills []string) map[string]string {
	if len(categories) == 0 || len(skills) == 0 {
		return map[string]string{}
	}
	wanted := map[string]string{}
	for _, skill := range skills {
		wanted[tailoringSkillKey(skill)] = skill
	}
	out := map[string]string{}
	for rawSkill, category := range categories {
		if display, ok := wanted[tailoringSkillKey(rawSkill)]; ok {
			out[display] = category
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
		if len(r.Alternatives) > 0 {
			out = append(out, strings.Join(r.Alternatives, " or "))
			continue
		}
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
	operation := s.Operation
	if operation != OperationAdd {
		operation = OperationRewrite
	}
	return Suggestion{
		Section:               s.Section,
		OriginalText:          s.OriginalText,
		SuggestedText:         s.SuggestedText,
		RequirementsAddressed: s.RequirementsAddressed,
		SkillsAdded:           s.SkillsAdded,
		KeywordsAdded:         s.KeywordsAdded,
		SkillCategories:       s.SkillCategories,
		Operation:             operation,
		TargetCompany:         s.TargetCompany,
		TargetTitle:           s.TargetTitle,
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
