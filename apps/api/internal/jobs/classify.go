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
	padded := " " + strings.Join(strings.Fields(value), " ") + " "

	// Exclusions are role-aware phrases/tokens rather than broad substrings.
	// In particular, do not exclude on "sales" alone: "Salesforce Developer"
	// is a software role while "Sales Engineer" is not part of this catalog.
	excluded := []string{
		" engineering manager ", " software engineering manager ", " development manager ",
		" manager, ", " manager ", " director ", " vice president ", " vp ", " head of ",
		" quality assurance ", " qa ", " tester ", " test engineer ", " sdet ",
		" analyst ", " product manager ", " program manager ", " project manager ", " scrum master ",
		" support engineer ", " solutions engineer ", " sales engineer ", " recruiter ",
		" mechanical engineer ", " civil engineer ", " electrical engineer ", " manufacturing engineer ",
		" industrial engineer ", " field engineer ", " process engineer ", " hardware engineer ",
		" quality engineer ", " validation engineer ",
	}
	for _, term := range excluded {
		if strings.Contains(padded, term) {
			return RoleClassification{Family: "EXCLUDED", Classification: "NON_SOFTWARE", Confidence: 0.98}
		}
	}

	// Specific role/stack families come before generic "developer" handling.
	// This intentionally includes language-specific titles that job boards use
	// instead of the generic "Software Engineer" label.
	families := []struct {
		terms []string
		name  string
	}{
		{[]string{" java full stack ", " java full-stack ", " full stack ", " fullstack ", " mern ", " mean stack "}, "FULLSTACK"},
		{[]string{" backend ", " back end ", " java backend ", " spring boot ", " golang ", " go developer ", " go engineer ", " node.js ", " nodejs ", " api engineer ", " api developer ", " microservices engineer ", " microservice engineer "}, "BACKEND"},
		{[]string{" frontend ", " front end ", " react developer ", " react engineer ", " angular developer ", " angular engineer ", " vue developer ", " vue engineer ", " ui developer "}, "FRONTEND"},
		{[]string{" platform ", " kubernetes engineer ", " container platform "}, "PLATFORM"},
		{[]string{" infrastructure ", " database engineer "}, "INFRASTRUCTURE"},
		{[]string{" site reliability ", " sre "}, "SRE"},
		{[]string{" devops ", " devsecops ", " build engineer ", " release engineer "}, "DEVOPS"},
		{[]string{" cloud engineer ", " cloud infrastructure "}, "CLOUD"},
		{[]string{" data engineer ", " analytics engineer "}, "DATA_ENGINEERING"},
		{[]string{" machine learning ", " ml engineer ", " mlops "}, "ML_ENGINEERING"},
		{[]string{" ai engineer ", " artificial intelligence ", " generative ai engineer ", " genai engineer "}, "AI_ENGINEERING"},
		{[]string{" security engineer ", " application security ", " product security engineer "}, "SECURITY_ENGINEERING"},
		{[]string{" ios ", " android ", " mobile "}, "MOBILE"},
		{[]string{" embedded ", " firmware "}, "EMBEDDED"},
		{[]string{" systems software ", " systems engineer, software ", " distributed systems engineer ", " distributed systems developer "}, "SYSTEMS"},
	}
	for _, family := range families {
		for _, term := range family.terms {
			if strings.Contains(padded, term) {
				return RoleClassification{Family: family.name, Classification: "IC_SOFTWARE", Confidence: 0.96}
			}
		}
	}

	softwareSpecific := []string{
		" software engineer ", " software developer ", " software development engineer ",
		" application engineer ", " application developer ", " web developer ",
		" java developer ", " java engineer ", " python developer ", " python engineer ",
		" c# developer ", " c# engineer ", " .net developer ", " .net engineer ",
		" javascript developer ", " typescript developer ", " ruby developer ", " ruby engineer ",
		" scala developer ", " scala engineer ", " kotlin developer ", " kotlin engineer ",
		" salesforce developer ", " servicenow developer ", " integration developer ",
	}
	for _, term := range softwareSpecific {
		if strings.Contains(padded, term) {
			return RoleClassification{Family: "SOFTWARE_ENGINEERING", Classification: "IC_SOFTWARE", Confidence: 0.95}
		}
	}

	// Retain broad developer titles when no exclusion/specialized family fired.
	if strings.Contains(padded, " developer ") ||
		strings.HasPrefix(value, "developer ") ||
		value == "developer" {
		return RoleClassification{Family: "SOFTWARE_ENGINEERING", Classification: "IC_SOFTWARE", Confidence: 0.90}
	}

	return RoleClassification{Family: "UNKNOWN", Classification: "UNKNOWN", Confidence: 0.30}
}
