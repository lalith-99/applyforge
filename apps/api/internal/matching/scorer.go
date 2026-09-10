package matching

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Score computes a deterministic Result for a candidate against a job.
func Score(in Input) Result {
	eligibility := CheckEligibility(in)

	transferableBySkill := indexTransferable(in.TransferableFromSkills)

	requiredMatched, requiredMissing, requiredTransfers, requiredCredit := coverSkills(in.RequiredSkills, in.CandidateSkills, transferableBySkill)
	preferredMatched, preferredMissing, preferredTransfers, preferredCredit := coverSkills(in.PreferredSkills, in.CandidateSkills, transferableBySkill)

	mustHaveCoverage := creditRatio(len(in.RequiredSkills), requiredCredit)
	preferredCoverage := creditRatio(len(in.PreferredSkills), preferredCredit)

	responsibilityRatio := responsibilityAlignment(
		in.Responsibilities,
		in.CandidateSkills,
		append(append([]SkillRequirement{}, in.RequiredSkills...), in.PreferredSkills...),
	)
	seniorityScore := seniorityAlignment(in.CandidateSeniority, in.JobSeniority)
	domainScore := domainAlignment(in.CandidateDomains, in.JobDomains)
	locationScore := locationAlignment(in)
	educationScore := educationAlignment(in)
	preferencesScore := preferencesAlignment(eligibility)

	// Keep these weights aligned with MASTER_REQUIREMENTS.md §20. They sum to
	// exactly 100, which makes each component independently auditable.
	components := ComponentScores{
		MustHaveSkillCoverage:   30 * mustHaveCoverage,
		ResponsibilityAlignment: 20 * responsibilityRatio,
		RoleSeniority:           15 * seniorityScore,
		PreferredSkills:         10 * preferredCoverage,
		DomainAlignment:         10 * domainScore,
		LocationWorkArrangement: 5 * locationScore,
		EducationCertifications: 5 * educationScore,
		CandidatePreferences:    5 * preferencesScore,
	}

	total := int(components.Total() + 0.5)
	if total > 100 {
		total = 100
	}
	if total < 0 {
		total = 0
	}

	allTransfers := mergeTransfers(requiredTransfers, preferredTransfers)

	// A skill can legitimately appear in both RequiredSkills and
	// PreferredSkills (imperfect JD parsing can classify the same skill
	// both ways) - dedupe so callers never see the same skill twice.
	matchedSkills := dedupeStrings(append(append([]string{}, requiredMatched...), preferredMatched...))
	sort.Strings(matchedSkills)

	currentMatch, targetMatch, suggestedAdditions := profileMatch(in, requiredMatched, requiredMissing, allTransfers)

	result := Result{
		TotalScore:               total,
		Grade:                    gradeFor(total),
		Components:               components,
		MatchedSkills:            matchedSkills,
		TransferableSkills:       allTransfers,
		MissingRequiredSkills:    requiredMissing,
		MissingPreferredSkills:   preferredMissing,
		PositiveEvidence:         positiveEvidence(matchedSkills, allTransfers),
		Concerns:                 concerns(requiredMissing, eligibility),
		OpportunityScore:         opportunityScore(total, in, eligibility),
		CurrentProfileMatch:      currentMatch,
		TargetProfileMatch:       targetMatch,
		SuggestedTargetAdditions: suggestedAdditions,
		Eligibility:              eligibility,
		ImmigrationRelevant:      immigrationRequired(in),
		ImmigrationPriorityScore: immigrationPriorityScore(eligibility.Immigration, immigrationRequired(in)),
	}

	switch result.Eligibility.Immigration.Status {
	case "SUPPORTED":
		result.PositiveEvidence = append(result.PositiveEvidence, "Job posting explicitly indicates immigration sponsorship/support.")
	case "HISTORICAL_SUPPORT":
		result.PositiveEvidence = append(result.PositiveEvidence, "Historical DOL immigration evidence: "+result.Eligibility.Immigration.Evidence)
	}

	result.Explanation = explanation(result, in)
	return result
}

func coverageRatio(total, matched int) float64 {
	if total == 0 {
		return 1.0
	}
	ratio := float64(matched) / float64(total)
	if ratio > 1 {
		ratio = 1
	}
	return ratio
}

// creditRatio is like coverageRatio but works with fractional credit (direct
// matches count as 1.0, transferable matches count for less — see coverSkills).
func creditRatio(total int, credit float64) float64 {
	if total == 0 {
		return 1.0
	}
	ratio := credit / float64(total)
	if ratio > 1 {
		ratio = 1
	}
	return ratio
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func indexTransferable(skills []TransferableSkill) map[string][]TransferableSkill {
	idx := make(map[string][]TransferableSkill)
	for _, t := range skills {
		key := strings.ToLower(t.TargetSkill)
		idx[key] = append(idx[key], t)
	}
	return idx
}

func requirementOptions(req SkillRequirement) []string {
	if len(req.Alternatives) > 0 {
		options := make([]string, 0, len(req.Alternatives))
		seen := map[string]bool{}
		for _, alternative := range req.Alternatives {
			key := strings.ToLower(strings.TrimSpace(alternative))
			if key != "" && !seen[key] {
				options = append(options, key)
				seen[key] = true
			}
		}
		if len(options) > 0 {
			return options
		}
	}
	key := strings.ToLower(strings.TrimSpace(req.NormalizedName))
	if key == "" {
		return nil
	}
	return []string{key}
}

func matchedRequirementOption(req SkillRequirement, skills map[string]bool) (string, bool) {
	for _, option := range requirementOptions(req) {
		if skills[option] {
			return option, true
		}
	}
	return "", false
}

// coverSkills partitions requirements into direct matches, transferable
// matches, and misses, and returns the fractional "credit" earned toward
// coverage: 1.0 per direct match, transferability_score/100 (capped below
// direct-match value) per transferable match, 0 for a miss. An alternative
// group counts as exactly one requirement and is satisfied by any one option.
func coverSkills(reqs []SkillRequirement, candidateSkills map[string]bool, transferable map[string][]TransferableSkill) (matched, missing []string, transfers []TransferableMatch, credit float64) {
	for _, req := range reqs {
		if option, ok := matchedRequirementOption(req, candidateSkills); ok {
			if len(req.Alternatives) > 0 {
				matched = append(matched, option)
			} else {
				matched = append(matched, req.NormalizedName)
			}
			credit += 1.0
			continue
		}

		var best *TransferableSkill
		for _, option := range requirementOptions(req) {
			candidate := bestTransfer(transferable[option])
			if candidate != nil && candidate.TransferabilityScore > 0 &&
				(best == nil || candidate.TransferabilityScore > best.TransferabilityScore) {
				best = candidate
			}
		}
		if best != nil {
			transfers = append(transfers, TransferableMatch{
				SourceSkill:        best.SourceSkill,
				TargetSkill:        req.NormalizedName,
				Level:              best.Level,
				PrepClassification: best.PrepClassification,
			})
			credit += transferCredit(best.TransferabilityScore)
			continue
		}

		missing = append(missing, req.NormalizedName)
	}
	return matched, missing, transfers, credit
}

// transferCredit converts a 0-100 transferability score into partial
// skill-coverage credit, capped at 0.8 so no transfer ever equals direct
// skill possession.
func transferCredit(transferabilityScore int) float64 {
	credit := float64(transferabilityScore) / 100
	if credit > 0.8 {
		credit = 0.8
	}
	return credit
}

func bestTransfer(candidates []TransferableSkill) *TransferableSkill {
	var best *TransferableSkill
	for i := range candidates {
		if best == nil || candidates[i].TransferabilityScore > best.TransferabilityScore {
			best = &candidates[i]
		}
	}
	return best
}

func mergeTransfers(a, b []TransferableMatch) []TransferableMatch {
	out := append([]TransferableMatch{}, a...)
	out = append(out, b...)
	return out
}

func responsibilityAlignment(
	responsibilities []string,
	candidateSkills map[string]bool,
	jobSkills []SkillRequirement,
) float64 {
	if len(responsibilities) == 0 {
		return 0.7 // no responsibilities extracted; neutral-leaning-positive default
	}

	// Score responsibility lines with explicit extracted technologies against
	// concrete candidate skills. Generic duties are not evidence of a gap.
	totalCredit := 0.0
	for _, resp := range responsibilities {
		lower := strings.ToLower(resp)
		mentionedReqs := make([]SkillRequirement, 0, 2)
		for _, req := range jobSkills {
			for _, option := range requirementOptions(req) {
				if option != "" && strings.Contains(lower, option) {
					mentionedReqs = append(mentionedReqs, req)
					break
				}
			}
		}

		if len(mentionedReqs) == 0 {
			totalCredit += 0.7
			continue
		}
		for _, req := range mentionedReqs {
			if _, ok := matchedRequirementOption(req, candidateSkills); ok {
				totalCredit += 1.0
				break
			}
		}
	}
	return totalCredit / float64(len(responsibilities))
}

func domainAlignment(candidateDomains, jobDomains []string) float64 {
	if len(jobDomains) == 0 {
		return 1.0
	}
	if len(candidateDomains) == 0 {
		return 0.7
	}
	matched := 0
	for _, jobDomain := range jobDomains {
		job := normalizeDomain(jobDomain)
		if job == "" {
			continue
		}
		for _, candidateDomain := range candidateDomains {
			candidate := normalizeDomain(candidateDomain)
			if candidate == "" {
				continue
			}
			if candidate == job || strings.Contains(candidate, job) || strings.Contains(job, candidate) {
				matched++
				break
			}
		}
	}
	return coverageRatio(len(jobDomains), matched)
}

func normalizeDomain(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

var seniorityOrder = map[string]int{
	"intern": 0, "entry-level": 1, "junior": 1, "mid": 2, "": 2,
	"senior": 3, "staff": 4, "lead": 4, "principal": 5,
}

func seniorityAlignment(candidate, job string) float64 {
	jobKey := strings.ToLower(strings.TrimSpace(job))
	candidateKey := strings.ToLower(strings.TrimSpace(candidate))
	if jobKey == "" || candidateKey == "" {
		return 0.7 // insufficient signal; neutral-leaning-positive default
	}

	jobRank, jobOK := seniorityOrder[jobKey]
	candidateRank, candOK := seniorityOrder[candidateKey]
	if !jobOK || !candOK {
		return 0.7
	}

	diff := jobRank - candidateRank
	if diff < 0 {
		diff = -diff
	}
	switch diff {
	case 0:
		return 1.0
	case 1:
		return 0.7
	default:
		return 0.35
	}
}

func locationAlignment(in Input) float64 {
	if !in.PreferredRemote && !in.PreferredHybrid && !in.PreferredOnsite {
		return 0.7 // no preference configured
	}
	switch in.RemoteType {
	case "remote":
		if in.PreferredRemote {
			return 1.0
		}
	case "onsite":
		if in.PreferredOnsite {
			return 1.0
		}
	case "hybrid":
		if in.PreferredHybrid {
			return 1.0
		}
	default:
		return 0.5
	}
	return 0.3
}

func educationAlignment(in Input) float64 {
	if !in.HasEducationReqs && !in.HasCertReqs {
		return 1.0
	}
	// We don't yet cross-reference resume education/certifications here
	// (deferred — see docs/MATCHING_ENGINE.md technical debt notes).
	return 0.6
}

func preferencesAlignment(eligibility EligibilityResult) float64 {
	if !eligibility.Eligible {
		return 0.0
	}
	if len(eligibility.Warnings) > 0 {
		return 0.6
	}
	// Historical DOL activity is useful, but it is weaker than explicit
	// role-level sponsorship language and therefore never receives full
	// preference credit.
	if eligibility.Immigration.Status == "HISTORICAL_SUPPORT" {
		return 0.85
	}
	return 1.0
}

func opportunityScore(matchScore int, in Input, eligibility EligibilityResult) int {
	if !eligibility.Eligible {
		return 0
	}

	freshness := freshnessScore(in.PostedAt, in.FirstSeenAt)
	preferences := preferencesAlignment(eligibility) * 100

	score := 0.75*float64(matchScore) + 0.15*freshness + 0.10*preferences
	return clampScore(score)
}

func freshnessScore(postedAt *time.Time, firstSeenAt time.Time) float64 {
	reference := firstSeenAt
	if postedAt != nil {
		reference = *postedAt
	}
	age := time.Since(reference)
	switch {
	case age <= time.Hour:
		return 100
	case age <= 6*time.Hour:
		return 90
	case age <= 24*time.Hour:
		return 75
	case age <= 3*24*time.Hour:
		return 55
	case age <= 7*24*time.Hour:
		return 35
	default:
		return 15
	}
}

func clampScore(v float64) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return int(v + 0.5)
}

func targetHasRequirement(req SkillRequirement, targetSkills map[string]bool) bool {
	_, ok := matchedRequirementOption(req, targetSkills)
	return ok
}

func profileMatch(in Input, requiredMatched, requiredMissing []string, transfers []TransferableMatch) (current, target int, suggestedAdditions []string) {
	current = clampScore(coverageRatio(len(in.RequiredSkills), len(requiredMatched)) * 100)

	// Target match additionally credits user-approved target skills and
	// meaningful transferable paths — never presented as current capability.
	targetCredit := len(requiredMatched)
	for _, missing := range requiredMissing {
		matchedTarget := false
		for _, req := range in.RequiredSkills {
			if strings.EqualFold(req.NormalizedName, missing) && targetHasRequirement(req, in.CandidateTargetSkills) {
				targetCredit++
				matchedTarget = true
				break
			}
		}
		if matchedTarget {
			continue
		}
		if in.CandidateTargetSkills[strings.ToLower(missing)] {
			targetCredit++
			continue
		}
		for _, t := range transfers {
			if strings.EqualFold(t.TargetSkill, missing) {
				targetCredit++
				suggestedAdditions = append(suggestedAdditions, missing)
				break
			}
		}
	}
	target = clampScore(coverageRatio(len(in.RequiredSkills), targetCredit) * 100)
	if target < current {
		target = current
	}
	return current, target, suggestedAdditions
}

func positiveEvidence(matched []string, transfers []TransferableMatch) []string {
	var evidence []string
	if len(matched) > 0 {
		evidence = append(evidence, fmt.Sprintf("Matches %d required/preferred skill(s) directly: %s", len(matched), strings.Join(matched, ", ")))
	}
	for _, t := range transfers {
		evidence = append(evidence, fmt.Sprintf("%s experience transfers toward %s (%s)", t.SourceSkill, t.TargetSkill, strings.ToLower(t.Level)))
	}
	return evidence
}

func concerns(missing []string, eligibility EligibilityResult) []string {
	var out []string
	if len(missing) > 0 {
		out = append(out, fmt.Sprintf("Missing required skill(s): %s", strings.Join(missing, ", ")))
	}
	out = append(out, eligibility.Warnings...)
	return out
}

func explanation(r Result, in Input) string {
	if !r.Eligibility.Eligible {
		return "This role does not meet your hard requirements: " + strings.Join(r.Eligibility.HardFailures, "; ")
	}
	return fmt.Sprintf(
		"%s match (%s) based on %d/%d required skills covered directly or via transferable experience, %s seniority alignment, and %s work-arrangement fit.",
		r.Grade, in.JobSeniority, len(in.RequiredSkills)-len(r.MissingRequiredSkills), len(in.RequiredSkills),
		describeRatio(r.Components.RoleSeniority/15), describeRatio(r.Components.LocationWorkArrangement/5),
	)
}

func describeRatio(ratio float64) string {
	switch {
	case ratio >= 0.9:
		return "strong"
	case ratio >= 0.6:
		return "reasonable"
	default:
		return "weak"
	}
}

func immigrationPriorityScore(assessment ImmigrationAssessment, relevant bool) int {
	if !relevant {
		return 50
	}
	switch assessment.Status {
	case "NOT_SUPPORTED":
		return 0
	case "SUPPORTED":
		// Role-level posting evidence is the strongest signal available.
		if assessment.EvidenceSource == "JOB_POSTING" && assessment.Confidence == "HIGH" {
			return 100
		}
		return 90
	case "HISTORICAL_SUPPORT":
		// Employer history is useful but must never equal an explicit
		// role-level sponsorship statement.
		switch {
		case assessment.H1BCertifiedCases >= 50:
			return 78
		case assessment.H1BCertifiedCases >= 10:
			return 72
		case assessment.H1BCertifiedCases >= 3:
			return 66
		default:
			return 60
		}
	default:
		// Unknown is intentionally not a rejection, but explicit/recent
		// sponsorship evidence should rank ahead of it for H-1B users.
		return 45
	}
}
