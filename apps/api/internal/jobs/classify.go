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
	management := []string{"engineering manager", "manager", "director", "vice president", " vp ", "head of"}
	excluded := []string{"quality assurance", " qa ", "tester", "test engineer", "sdet", "analyst", "product manager", "program manager", "project manager", "scrum master", "support engineer", "recruiter", "sales"}
	for _, term := range append(management, excluded...) {
		if strings.Contains(" "+value+" ", term) {
			return RoleClassification{Family: "EXCLUDED", Classification: "NON_SOFTWARE", Confidence: 0.98}
		}
	}
	for _, family := range []struct{ term, name string }{
		{"backend", "BACKEND"}, {"front end", "FRONTEND"}, {"frontend", "FRONTEND"}, {"full stack", "FULLSTACK"},
		{"platform", "PLATFORM"}, {"infrastructure", "INFRASTRUCTURE"}, {"site reliability", "SRE"}, {"sre", "SRE"},
		{"devops", "DEVOPS"}, {"cloud", "CLOUD"}, {"data engineer", "DATA_ENGINEERING"}, {"machine learning", "ML_ENGINEERING"},
		{" ai ", "AI_ENGINEERING"}, {"security", "SECURITY_ENGINEERING"}, {"ios", "MOBILE"}, {"android", "MOBILE"},
		{"mobile", "MOBILE"}, {"embedded", "EMBEDDED"}, {"systems", "SYSTEMS"},
	} {
		if strings.Contains(" "+value+" ", family.term) {
			return RoleClassification{Family: family.name, Classification: "IC_SOFTWARE", Confidence: 0.95}
		}
	}
	if strings.Contains(value, "software engineer") || strings.Contains(value, "software developer") || strings.Contains(value, "developer") || strings.Contains(value, "engineer") {
		return RoleClassification{Family: "SOFTWARE_ENGINEERING", Classification: "IC_SOFTWARE", Confidence: 0.85}
	}
	return RoleClassification{Family: "UNKNOWN", Classification: "UNKNOWN", Confidence: 0.3}
}
