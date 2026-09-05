package matching

import "strings"

// CheckEligibility computes hard failures/warnings before scoring. Ambiguous
// requirements become warnings, never automatic failures (see
// MASTER_REQUIREMENTS.md §19).
func CheckEligibility(in Input) EligibilityResult {
	result := EligibilityResult{Eligible: true}

	for _, company := range in.ExcludedCompanies {
		if strings.EqualFold(strings.TrimSpace(company), strings.TrimSpace(in.CompanyName)) {
			result.Eligible = false
			result.HardFailures = append(result.HardFailures, "employer is on your excluded companies list")
		}
	}

	for _, loc := range in.ExcludedLocations {
		if loc != "" && in.LocationText != "" && strings.Contains(strings.ToLower(in.LocationText), strings.ToLower(loc)) {
			result.Eligible = false
			result.HardFailures = append(result.HardFailures, "location is on your excluded locations list")
		}
	}

	if len(in.PreferredEmploymentTypes) > 0 && in.EmploymentType != "" {
		matched := false
		for _, t := range in.PreferredEmploymentTypes {
			if normalizeEmploymentType(t) == normalizeEmploymentType(in.EmploymentType) {
				matched = true
				break
			}
		}
		if !matched {
			result.Eligible = false
			result.HardFailures = append(result.HardFailures, "employment type does not match your preferences")
		}
	}

	if !in.PreferredRemote && !in.PreferredHybrid && !in.PreferredOnsite {
		// No work-arrangement preference configured; nothing to check.
	} else {
		switch in.RemoteType {
		case "remote":
			if !in.PreferredRemote {
				result.Warnings = append(result.Warnings, "this role is remote, which is outside your stated preferences")
			}
		case "onsite":
			if !in.PreferredOnsite {
				result.Warnings = append(result.Warnings, "this role is onsite, which is outside your stated preferences")
			}
		case "hybrid":
			if !in.PreferredHybrid {
				result.Warnings = append(result.Warnings, "this role is hybrid, which is outside your stated preferences")
			}
		default:
			result.Warnings = append(result.Warnings, "work arrangement (remote/hybrid/onsite) is unclear from the posting")
		}
	}

	return result
}

func normalizeEmploymentType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	switch t {
	case "fulltime", "full_time", "full-time", "full time":
		return "full_time"
	case "contract", "contractor":
		return "contract"
	case "internship", "intern":
		return "internship"
	default:
		return t
	}
}
