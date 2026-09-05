package tailoring

import "strings"

// ComputeAlignment computes the deterministic Resume Alignment Score (see
// MASTER_REQUIREMENTS.md §23): how closely a specific resume's skill set
// reflects a job's stated requirements. This is intentionally distinct from
// the Job Match Score in internal/matching, and is never called an "ATS
// score" or expressed as a pass probability.
func ComputeAlignment(skillSet map[string]bool, requiredSkills, preferredSkills, responsibilities []string) int {
	requiredCoverage := coverage(requiredSkills, skillSet)
	preferredCoverage := coverage(preferredSkills, skillSet)
	responsibilityCoverage := responsibilityMatch(responsibilities, skillSet)

	score := 100 * (0.5*requiredCoverage + 0.2*preferredCoverage + 0.3*responsibilityCoverage)
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return int(score + 0.5)
}

func coverage(skills []string, have map[string]bool) float64 {
	if len(skills) == 0 {
		return 1.0
	}
	matched := 0
	for _, s := range skills {
		if have[strings.ToLower(s)] {
			matched++
		}
	}
	return float64(matched) / float64(len(skills))
}

func responsibilityMatch(responsibilities []string, have map[string]bool) float64 {
	if len(responsibilities) == 0 {
		return 0.7
	}
	matched := 0
	for _, resp := range responsibilities {
		lower := strings.ToLower(resp)
		for skill := range have {
			if skill != "" && strings.Contains(lower, skill) {
				matched++
				break
			}
		}
	}
	return float64(matched) / float64(len(responsibilities))
}
