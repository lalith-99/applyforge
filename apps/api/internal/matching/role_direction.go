package matching

import "strings"

// recommendationRoleDirectionEligible cheaply rejects only high-confidence
// specialty mismatches before requirement parsing. Generic software titles and
// adjacent backend/frontend/platform/cloud/devops roles remain eligible so the
// deterministic scorer can decide them with richer job-description evidence.
func recommendationRoleDirectionEligible(title string, targetRoles []string) bool {
	if len(targetRoles) == 0 {
		return true
	}

	hasDirectionalTarget := false
	targetSpecialties := make(map[string]bool)
	for _, target := range targetRoles {
		if recommendationHasDirectionalTarget(target) {
			hasDirectionalTarget = true
		}
		if specialty, ok := recommendationHighConfidenceSpecialty(target); ok {
			targetSpecialties[specialty] = true
		}
	}
	if !hasDirectionalTarget {
		// A profile that only says something broad such as "Software Engineer"
		// is not enough evidence to exclude a specialty before parsing the JD.
		return true
	}

	jobSpecialty, known := recommendationHighConfidenceSpecialty(title)
	if !known {
		return true
	}
	return targetSpecialties[jobSpecialty]
}

func recommendationHasDirectionalTarget(value string) bool {
	lower := recommendationNormalizeRoleText(value)
	for _, marker := range []string{
		"backend", "back end", "java", "golang", "go developer", "spring",
		"full stack", "fullstack", "frontend", "front end", "react", "angular",
		"devops", "platform", "cloud", "site reliability", "sre",
		"data engineer", "analytics engineer", "machine learning", "ml engineer",
		"security", "cyber", "ios", "android", "mobile", "embedded", "firmware",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// recommendationHighConfidenceSpecialty deliberately recognizes only
// specialties that are usually distinct career tracks. We do not classify
// backend, frontend, full-stack, platform, cloud, or DevOps here because those
// paths are commonly transferable for general software candidates and should
// remain available to the richer scorer.
func recommendationHighConfidenceSpecialty(value string) (string, bool) {
	lower := recommendationNormalizeRoleText(value)

	switch {
	case strings.Contains(lower, "site reliability"), recommendationContainsRoleToken(lower, "sre"):
		return "sre", true
	case strings.Contains(lower, "application security"),
		strings.Contains(lower, "product security"),
		strings.Contains(lower, "security engineer"),
		strings.Contains(lower, "cybersecurity"),
		strings.Contains(lower, "cyber security"),
		strings.Contains(lower, "devsecops"):
		return "security", true
	case strings.Contains(lower, "machine learning"),
		strings.Contains(lower, "ml engineer"),
		strings.Contains(lower, "artificial intelligence engineer"),
		strings.Contains(lower, "ai engineer"):
		return "ml", true
	case strings.Contains(lower, "data engineer"),
		strings.Contains(lower, "data platform engineer"),
		strings.Contains(lower, "analytics engineer"),
		strings.Contains(lower, "etl developer"):
		return "data", true
	case strings.Contains(lower, "ios engineer"),
		strings.Contains(lower, "ios developer"),
		strings.Contains(lower, "android engineer"),
		strings.Contains(lower, "android developer"),
		strings.Contains(lower, "mobile engineer"),
		strings.Contains(lower, "mobile developer"):
		return "mobile", true
	case strings.Contains(lower, "embedded software"),
		strings.Contains(lower, "embedded engineer"),
		strings.Contains(lower, "firmware engineer"),
		strings.Contains(lower, "firmware developer"):
		return "embedded", true
	default:
		return "", false
	}
}

func recommendationNormalizeRoleText(value string) string {
	value = strings.ToLower(value)
	value = strings.NewReplacer("-", " ", "/", " ", "_", " ", "|", " ", ",", " ", ":", " ", "(", " ", ")", " ").Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func recommendationContainsRoleToken(value, token string) bool {
	for _, field := range strings.Fields(value) {
		if field == token {
			return true
		}
	}
	return false
}
