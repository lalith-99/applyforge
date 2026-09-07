package jobs

import "strings"

type RoleClassification struct {
	Family         string
	Classification string
	Confidence     float32
}

func classifyTitle(title string) RoleClassification {
	value := strings.ToLower(strings.TrimSpace(title))
	if value == "" {
		return RoleClassification{Family: "UNKNOWN", Classification: "UNKNOWN"}
	}
	padded := " " + value + " "

	excluded := []string{
		"engineering manager", "manager", "director", "vice president", " vp ", "head of",
		"quality assurance", " qa ", "tester", "test engineer", "sdet", "analyst",
		"product manager", "program manager", "project manager", "scrum master",
		"support engineer", "solutions engineer", "sales engineer", "recruiter", "sales",
		"mechanical engineer", "civil engineer", "electrical engineer", "manufacturing engineer",
		"industrial engineer", "field engineer", "process engineer", "hardware engineer",
		"quality engineer", "validation engineer",
	}
	for _, term := range excluded {
		if strings.Contains(padded, term) {
			return RoleClassification{Family: "EXCLUDED", Classification: "NON_SOFTWARE", Confidence: 0.98}
		}
	}

	for _, family := range []struct{ term, name string }{
		{"backend", "BACKEND"},
		{"back end", "BACKEND"},
		{"front end", "FRONTEND"},
		{"frontend", "FRONTEND"},
		{"full stack", "FULLSTACK"},
		{"fullstack", "FULLSTACK"},
		{"platform", "PLATFORM"},
		{"infrastructure", "INFRASTRUCTURE"},
		{"site reliability", "SRE"},
		{" sre ", "SRE"},
		{"devops", "DEVOPS"},
		{"devsecops", "DEVOPS"},
		{"cloud engineer", "CLOUD"},
		{"data engineer", "DATA_ENGINEERING"},
		{"machine learning", "ML_ENGINEERING"},
		{" ml engineer", "ML_ENGINEERING"},
		{" ai engineer", "AI_ENGINEERING"},
		{"artificial intelligence", "AI_ENGINEERING"},
		{"security engineer", "SECURITY_ENGINEERING"},
		{"application security", "SECURITY_ENGINEERING"},
		{"ios", "MOBILE"},
		{"android", "MOBILE"},
		{"mobile", "MOBILE"},
		{"embedded", "EMBEDDED"},
		{"firmware", "EMBEDDED"},
		{"systems software", "SYSTEMS"},
		{"systems engineer, software", "SYSTEMS"},
		{"database engineer", "INFRASTRUCTURE"},
		{"application engineer", "SOFTWARE_ENGINEERING"},
		{"application developer", "SOFTWARE_ENGINEERING"},
		{"web developer", "SOFTWARE_ENGINEERING"},
	} {
		if strings.Contains(padded, family.term) {
			return RoleClassification{Family: family.name, Classification: "IC_SOFTWARE", Confidence: 0.95}
		}
	}

	if strings.Contains(value, "software engineer") ||
		strings.Contains(value, "software developer") ||
		strings.Contains(value, "software development engineer") ||
		strings.Contains(value, " developer") ||
		strings.HasPrefix(value, "developer") {
		return RoleClassification{Family: "SOFTWARE_ENGINEERING", Classification: "IC_SOFTWARE", Confidence: 0.9}
	}

	return RoleClassification{Family: "UNKNOWN", Classification: "UNKNOWN", Confidence: 0.3}
}
