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

	responsibilityRatio := responsibilityAlignment(in.Responsibilities, in.CandidateSkills)
	seniorityScore := seniorityAlignment(in.CandidateSeniority, in.JobSeniority)
	locationScore := locationAlignment(in)
	educationScore := educationAlignment(in)
	preferencesScore := preferencesAlignment(eligibility)

	components := ComponentScores{
		MustHaveSkillCoverage:   30 * mustHaveCoverage,
		ResponsibilityAlignment: 20 * responsibilityRatio,
		RoleSeniority:           15 * seniorityScore,
		PreferredSkills:         10 * preferredCoverage,
		DomainAlignment:         10 * 0.7, // no reliable domain signal from the heuristic JD parser yet
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

// coverSkills partitions requirements into direct matches, transferable
// matches, and misses, and returns the fractional "credit" earned toward
// coverage: 1.0 per direct match, transferability_score/100 (capped below
// direct-match value) per transferable match, 0 for a miss. Transferable
// credit intentionally never reaches 1.0 — it must always score lower than
// actually having the skill (see MASTER_REQUIREMENTS.md §24).
func coverSkills(reqs []SkillRequirement, candidateSkills map[string]bool, transferable map[string][]TransferableSkill) (matched, missing []string, transfers []TransferableMatch, credit float64) {
	for _, req := range reqs {
		key := strings.ToLower(req.NormalizedName)
		if candidateSkills[key] {
			matched = append(matched, req.NormalizedName)
			credit += 1.0
			continue
		}

		if best := bestTransfer(transferable[key]); best != nil && best.TransferabilityScore > 0 {
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

func responsibilityAlignment(responsibilities []string, candidateSkills map[string]bool) float64 {
	if len(responsibilities) == 0 {
		return 0.7 // no responsibilities extracted; neutral-leaning-positive default
	}
	matched := 0
	for _, resp := range responsibilities {
		lower := strings.ToLower(resp)
		for skill := range candidateSkills {
			if skill != "" && strings.Contains(lower, skill) {
				matched++
				break
			}
		}
	}
	return coverageRatio(len(responsibilities), matched)
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

func profileMatch(in Input, requiredMatched, requiredMissing []string, transfers []TransferableMatch) (current, target int, suggestedAdditions []string) {
	current = clampScore(coverageRatio(len(in.RequiredSkills), len(requiredMatched)) * 100)

	// Target match additionally credits user-approved target skills and
	// meaningful transferable paths — never presented as current capability.
	targetCredit := len(requiredMatched)
	for _, missing := range requiredMissing {
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
