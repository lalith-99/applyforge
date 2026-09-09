package matching

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/candidateprofile"
	"github.com/lalithlochan/applyforge/apps/api/internal/candidateskills"
	immigrationdata "github.com/lalithlochan/applyforge/apps/api/internal/immigration"
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
	immigrationRepo   *immigrationdata.Repository
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

// WithImmigrationEvidence adds historical employer-level DOL evidence to
// role matching without changing the base constructor used by existing tests.
func (s *Service) WithImmigrationEvidence(repo *immigrationdata.Repository) *Service {
	s.immigrationRepo = repo
	return s
}

// candidateMatchContext contains user-specific inputs shared by every job in a
// recommendation run. Loading it once avoids re-reading the same candidate
// skills, preferences, profile, and transferability graph for every job.
type candidateMatchContext struct {
	skills       map[string]bool
	targetSkills map[string]bool
	transferable []TransferableSkill
	prefs        preferences.Preferences
	profile      profile.Profile
}

// Match computes (and caches) the deterministic match Result for a user against a job.
func (s *Service) Match(ctx context.Context, jobID, userID uuid.UUID) (Result, error) {
	job, err := s.jobsRepo.GetByID(ctx, jobID)
	if err != nil {
		return Result{}, err
	}

	candidateCtx, err := s.loadCandidateMatchContext(ctx, userID)
	if err != nil {
		return Result{}, err
	}
	return s.matchJobWithContext(ctx, job, userID, candidateCtx)
}

func (s *Service) loadCandidateMatchContext(ctx context.Context, userID uuid.UUID) (candidateMatchContext, error) {
	skills, err := s.candidateSkills.ListForUser(ctx, userID)
	if err != nil {
		return candidateMatchContext{}, err
	}

	prefs, err := s.preferencesRepo.Get(ctx, userID)
	if err != nil && err != preferences.ErrNotFound {
		return candidateMatchContext{}, err
	}

	candidateProfile, err := s.profileRepo.Get(ctx, userID)
	if err != nil && err != profile.ErrNotFound {
		return candidateMatchContext{}, err
	}

	candidateSkillSet, candidateTargetSkillSet, transferableSkillKeys := buildCandidateSkillSets(skills)
	transferable, err := s.repo.ListTransferableFromSkills(ctx, transferableSkillKeys)
	if err != nil {
		return candidateMatchContext{}, err
	}

	return candidateMatchContext{
		skills:       candidateSkillSet,
		targetSkills: candidateTargetSkillSet,
		transferable: transferable,
		prefs:        prefs,
		profile:      candidateProfile,
	}, nil
}

func buildCandidateSkillSets(skills []candidateskills.Skill) (current, target map[string]bool, transferableKeys []string) {
	current = make(map[string]bool, len(skills))
	target = make(map[string]bool)
	transferableKeys = make([]string, 0, len(skills))

	for _, sk := range skills {
		key := strings.ToLower(strings.TrimSpace(sk.NormalizedName))
		if key == "" {
			continue
		}
		if sk.Status == candidateskills.StatusUserApproved || sk.Status == candidateskills.StatusTargetSkill {
			target[key] = true
			continue
		}
		current[key] = true
		transferableKeys = append(transferableKeys, key)
	}
	return current, target, transferableKeys
}

func (s *Service) matchJobWithContext(
	ctx context.Context,
	job jobs.Job,
	userID uuid.UUID,
	candidateCtx candidateMatchContext,
) (Result, error) {
	reqs, err := s.requirementsSvc.GetOrParse(ctx, job.ID, job.Title, job.Description, job.ContentHash)
	if err != nil {
		return Result{}, err
	}

	var companyEvidence immigrationdata.CompanyEvidence
	if s.immigrationRepo != nil {
		companyEvidence, err = s.immigrationRepo.GetForCompany(ctx, job.CompanyID, job.CompanyName)
		if err != nil {
			// Historical evidence is a secondary signal. A temporary evidence
			// lookup failure must never prevent a candidate from viewing or
			// matching against an otherwise valid job.
			slog.Warn("company immigration evidence lookup failed", "company_id", job.CompanyID, "company_name", job.CompanyName, "error", err)
		}
	}

	input := Input{
		CandidateSkills:                     candidateCtx.skills,
		CandidateTargetSkills:               candidateCtx.targetSkills,
		TransferableFromSkills:              candidateCtx.transferable,
		CandidateSeniority:                  stringOrEmpty(candidateCtx.profile.Seniority),
		CandidateDomains:                    candidateCtx.profile.PreferredIndustries,
		PreferredRemote:                     candidateCtx.prefs.Remote,
		PreferredHybrid:                     candidateCtx.prefs.Hybrid,
		PreferredOnsite:                     candidateCtx.prefs.Onsite,
		PreferredEmploymentTypes:            candidateCtx.prefs.EmploymentTypes,
		ExcludedCompanies:                   candidateCtx.prefs.ExcludedCompanies,
		ExcludedLocations:                   candidateCtx.prefs.ExcludedLocations,
		RequiresH1BTransfer:                 candidateCtx.prefs.RequiresH1BTransfer,
		RequiresNewH1BCapSponsorship:        candidateCtx.prefs.RequiresNewH1BCapSponsorship,
		RequiresFutureEmploymentSponsorship: candidateCtx.prefs.RequiresFutureEmploymentSponsorship,
		GreenCardSupportPreferred:           candidateCtx.prefs.GreenCardSupportPreferred,
		GreenCardSupportRequired:            candidateCtx.prefs.GreenCardSupportRequired,
		PermSupportPreferred:                candidateCtx.prefs.PermSupportPreferred,
		WorkAuthorization:                   stringOrEmpty(candidateCtx.prefs.WorkAuthorization),
		ImmigrationStatus:                   stringOrEmpty(candidateCtx.prefs.ImmigrationStatus),
		CompanyH1BCertifiedCases:            companyEvidence.H1BCertified,
		CompanyH1BTotalCases:                companyEvidence.H1BTotal,
		CompanyPERMCertifiedCases:           companyEvidence.PERMCertified,
		CompanyPERMTotalCases:               companyEvidence.PERMTotal,
		CompanyEvidenceLatestFY:             companyEvidence.LatestFiscalYear,
		CompanyEvidenceEmployers:            companyEvidence.MatchedEmployers,
		CompanyName:                         job.CompanyName,
		LocationText:                        stringOrEmpty(job.LocationText),
		RemoteType:                          stringOrEmpty(job.RemoteType),
		EmploymentType:                      stringOrEmpty(job.EmploymentType),
		JobSeniority:                        stringOrEmpty(reqs.Seniority),
		RequiredSkills:                      toSkillRequirements(reqs.RequiredSkills),
		PreferredSkills:                     toSkillRequirements(reqs.PreferredSkills),
		Responsibilities:                    reqs.Responsibilities,
		JobDomains:                          reqs.Domains,
		HasEducationReqs:                    len(reqs.EducationRequirements) > 0,
		HasCertReqs:                         len(reqs.Certifications) > 0,
		PostedAt:                            job.PostedAt,
		FirstSeenAt:                         job.FirstSeenAt,
		JobDescription:                      job.Description,
		WorkAuthorizationRequirements:       stringOrEmpty(reqs.WorkAuthorizationRequirements),
	}

	result := Score(input)
	if err := s.repo.Save(ctx, job.ID, userID, result); err != nil {
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
// retrieval -> deterministic scoring. Returns up to limit ELIGIBLE jobs from
// the last 24 hours for userID, ranked by Result.TotalScore before the later
// daily-priority/AI reranking stage. Semantic retrieval is optional: when a
// candidate embedding is unavailable, the funnel still uses target-role/skill
// lexical retrieval. ErrNotFound now means there is no generated candidate
// profile at all.
func (s *Service) Recommend(ctx context.Context, userID uuid.UUID, limit int) ([]RankedJob, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	candidateProfile, err := s.candidateProfiles.GetLatest(ctx, userID)
	if err != nil {
		return nil, err
	}

	embedding, embeddingErr := s.candidateProfiles.GetLatestEmbedding(ctx, userID)
	if embeddingErr != nil && !errors.Is(embeddingErr, candidateprofile.ErrNotFound) {
		return nil, embeddingErr
	}

	candidateCtx, err := s.loadCandidateMatchContext(ctx, userID)
	if err != nil {
		return nil, err
	}
	prefs := candidateCtx.prefs

	const recommendedJobMaxAge = 24 * time.Hour
	postedAfter := time.Now().UTC().Add(-recommendedJobMaxAge)
	requiresH1BSupport := preferences.RequiresH1BSupport(prefs)
	filter := jobs.EmbeddingSearchFilter{
		CountryCode:              "US",
		PostedAfter:              &postedAfter,
		ExcludeSponsorshipDenied: requiresH1BSupport,
		RequireRecentH1BHistory:  requiresH1BSupport,
	}
	if len(prefs.EmploymentTypes) == 1 && normalizeEmploymentType(prefs.EmploymentTypes[0]) == "full_time" {
		filter.EmploymentType = "FullTime"
	}
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
	var matches []jobs.JobMatch
	if embeddingErr == nil {
		matches, err = s.jobsRepo.SearchByEmbedding(ctx, embedding, poolSize, filter)
		if err != nil {
			return nil, err
		}
	} else {
		slog.Info("candidate embedding unavailable; using lexical recommendation fallback", "user_id", userID)
	}

	candidateByID := make(map[uuid.UUID]jobs.Job, int(poolSize)+200)
	candidateOrder := make([]uuid.UUID, 0, int(poolSize)+200)
	addCandidate := func(job jobs.Job) {
		if _, exists := candidateByID[job.ID]; exists {
			return
		}
		candidateByID[job.ID] = job
		candidateOrder = append(candidateOrder, job.ID)
	}
	for _, match := range matches {
		addCandidate(match.Job)
	}

	// Hybrid retrieval: semantic nearest neighbors are excellent for broad
	// relevance, but exact target-title/technology roles can otherwise be
	// crowded out in a large catalog. Union a bounded lexical pool so titles
	// such as Java Full Stack Developer or Golang Developer always get a
	// chance to be deterministically scored.
	{
		lexicalFilter := jobs.ListFilter{
			RemoteType:               filter.RemoteType,
			EmploymentType:           filter.EmploymentType,
			PostedAfter:              filter.PostedAfter,
			CountryCode:              filter.CountryCode,
			ExcludeSponsorshipDenied: filter.ExcludeSponsorshipDenied,
			RequireRecentH1BHistory:  filter.RequireRecentH1BHistory,
			Sort:                     "newest",
			Limit:                    40,
		}
		const maxHybridCandidates = 700
		for _, term := range recommendationLexicalTerms(candidateProfile) {
			lexicalFilter.Search = term
			found, _, listErr := s.jobsRepo.List(ctx, lexicalFilter)
			if listErr != nil {
				slog.Warn("lexical recommendation retrieval failed", "term", term, "user_id", userID, "error", listErr)
				continue
			}
			for _, job := range found {
				addCandidate(job)
				if len(candidateOrder) >= maxHybridCandidates {
					break
				}
			}
			if len(candidateOrder) >= maxHybridCandidates {
				break
			}
		}
	}

	retrievedCandidates := len(candidateOrder)
	eligibleCandidates := 0
	seniorityRejected := 0
	ranked := make([]RankedJob, 0, len(candidateOrder))
	for _, jobID := range candidateOrder {
		job := candidateByID[jobID]
		result, err := s.matchJobWithContext(ctx, job, userID, candidateCtx)
		if err != nil {
			slog.Error("match failed during recommend", "job_id", job.ID, "user_id", userID, "error", err)
			continue
		}
		if !result.Eligibility.Eligible {
			continue
		}
		eligibleCandidates++
		if !recommendationSeniorityEligible(job.Title, candidateProfile) {
			seniorityRejected++
			continue
		}
		ranked = append(ranked, RankedJob{Job: job, Result: result})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		left := recommendationPreAIRank(ranked[i], candidateProfile)
		right := recommendationPreAIRank(ranked[j], candidateProfile)
		if left == right {
			return ranked[i].Result.TotalScore > ranked[j].Result.TotalScore
		}
		return left > right
	})
	preAICandidates := len(ranked)
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	slog.Info("recommendation funnel completed",
		"user_id", userID,
		"retrieved_candidates", retrievedCandidates,
		"eligible_candidates", eligibleCandidates,
		"seniority_rejected", seniorityRejected,
		"pre_ai_candidates", preAICandidates,
		"returned_candidates", len(ranked),
	)
	return ranked, nil
}

func recommendationLexicalTerms(candidate candidateprofile.Profile) []string {
	seen := make(map[string]bool)
	terms := make([]string, 0, 12)
	add := func(value string) {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" || seen[key] {
			return
		}
		seen[key] = true
		terms = append(terms, value)
	}

	for _, role := range candidate.TargetRoles {
		add(role)
		if len(terms) >= 8 {
			break
		}
	}

	titleSkill := map[string]string{
		"java":        "Java",
		"spring boot": "Spring Boot",
		"golang":      "Golang",
		"go":          "Golang",
		"python":      "Python",
		".net":        ".NET",
		"c#":          "C#",
		"react":       "React",
		"angular":     "Angular",
		"kubernetes":  "Kubernetes",
		"devops":      "DevOps",
	}
	for _, skill := range append(append([]string{}, candidate.CoreSkills...), candidate.SecondarySkills...) {
		if term, ok := titleSkill[strings.ToLower(strings.TrimSpace(skill))]; ok {
			add(term)
		}
		if len(terms) >= 12 {
			break
		}
	}
	return terms
}

var recommendationGenericRoleTokens = map[string]bool{
	"software": true, "engineer": true, "engineering": true, "developer": true,
	"development": true, "senior": true, "sr": true, "mid": true, "level": true,
	"ii": true, "iii": true, "iv": true, "lead": true, "staff": true, "principal": true,
}

func recommendationPreAIRank(r RankedJob, candidate candidateprofile.Profile) float64 {
	roleScore := recommendationTargetRoleScore(r.Job.Title, candidate.TargetRoles)
	// Technical evidence stays the majority signal, but target-role direction
	// is strong enough to stop generic high-overlap infrastructure roles from
	// crowding Java/backend/full-stack targets out before the AI stage.
	return 0.70*float64(r.Result.TotalScore) + 0.30*roleScore
}

func recommendationTargetRoleScore(title string, targetRoles []string) float64 {
	if len(targetRoles) == 0 {
		return 70
	}
	titleTokens := recommendationRoleTokenSet(title)
	best := 35.0
	for _, target := range targetRoles {
		targetTokens := recommendationRoleTokenSet(target)
		if len(targetTokens) == 0 {
			// A generic target such as "Software Engineer" should be neutral
			// rather than penalizing every specialization.
			if strings.Contains(strings.ToLower(title), "software") ||
				strings.Contains(strings.ToLower(title), "developer") ||
				strings.Contains(strings.ToLower(title), "engineer") {
				if best < 70 {
					best = 70
				}
			}
			continue
		}
		matched := 0
		for token := range targetTokens {
			if titleTokens[token] {
				matched++
			}
		}
		if matched == 0 {
			continue
		}
		coverage := float64(matched) / float64(len(targetTokens))
		score := 60 + 40*coverage
		if score > best {
			best = score
		}
	}
	return best
}

func recommendationRoleTokenSet(value string) map[string]bool {
	value = strings.ToLower(value)
	value = strings.NewReplacer("-", " ", "/", " ", "_", " ", ".", " ").Replace(value)
	out := map[string]bool{}
	for _, token := range strings.Fields(value) {
		if recommendationGenericRoleTokens[token] {
			continue
		}
		switch token {
		case "golang":
			token = "go"
		case "fullstack":
			token = "full"
			out["stack"] = true
		case "microservices":
			token = "microservice"
		}
		if len(token) >= 2 {
			out[token] = true
		}
	}
	return out
}

func recommendationSeniorityEligible(title string, candidate candidateprofile.Profile) bool {
	jobRank, jobKnown := recommendationSeniorityRank(title)
	if !jobKnown {
		return true
	}

	candidateRank, candidateKnown := 0, false
	if candidate.Seniority != nil {
		candidateRank, candidateKnown = recommendationSeniorityRank(*candidate.Seniority)
	}
	if !candidateKnown && candidate.YearsExperience != nil {
		switch {
		case *candidate.YearsExperience >= 5:
			candidateRank, candidateKnown = 3, true // senior
		case *candidate.YearsExperience >= 2:
			candidateRank, candidateKnown = 2, true // mid
		default:
			candidateRank, candidateKnown = 1, true
		}
	}
	if !candidateKnown {
		// We can still reject Staff/Principal unless the target itself asks for
		// that level; unknown candidate seniority is not evidence for elite IC levels.
		if jobRank >= 4 && !candidateTargetsSeniority(candidate.TargetRoles, jobRank) {
			return false
		}
		return true
	}

	if jobRank > candidateRank && !candidateTargetsSeniority(candidate.TargetRoles, jobRank) {
		return false
	}
	// A senior+ candidate should not get junior/entry recommendations simply
	// because the technologies happen to overlap.
	if candidateRank >= 3 && jobRank <= 1 {
		return false
	}
	return true
}

func candidateTargetsSeniority(targetRoles []string, minimumRank int) bool {
	for _, target := range targetRoles {
		if rank, ok := recommendationSeniorityRank(target); ok && rank >= minimumRank {
			return true
		}
	}
	return false
}

func recommendationSeniorityRank(value string) (int, bool) {
	lower := strings.ToLower(value)
	switch {
	case strings.Contains(lower, "principal"), strings.Contains(lower, "distinguished"):
		return 5, true
	case strings.Contains(lower, "staff"), strings.Contains(lower, "lead"):
		return 4, true
	case strings.Contains(lower, "senior"), strings.Contains(lower, "sr. "), strings.HasPrefix(lower, "sr "):
		return 3, true
	case strings.Contains(lower, "mid"), strings.Contains(lower, "level 2"), strings.Contains(lower, "level ii"):
		return 2, true
	case strings.Contains(lower, "junior"), strings.Contains(lower, "entry"), strings.Contains(lower, "jr. "), strings.HasPrefix(lower, "jr "):
		return 1, true
	case strings.Contains(lower, "intern"):
		return 0, true
	default:
		return 0, false
	}
}
