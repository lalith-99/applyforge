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

	// ATS titles frequently separate role words with punctuation (for example
	// "Manager/Software Engineering" or "QA|Automation Engineer"). Normalize
	// separators before cheap classification so obvious non-target roles do not
	// fall through to AI classification/enrichment just because of typography.
	normalized := strings.NewReplacer(
		"/", " ", "|", " ", ",", " ", ":", " ", ";", " ",
		"(", " ", ")", " ", "[", " ", "]", " ", "{", " ", "}", " ",
	).Replace(value)
	padded := " " + strings.Join(strings.Fields(normalized), " ") + " "

	// Exclusions are role-aware phrases/tokens rather than broad substrings.
	// In particular, do not exclude on "sales" alone: "Salesforce Developer"
	// is a software role while "Sales Engineer" is not part of this catalog.
	// Keep this list deliberately high-confidence because EXCLUDED titles skip
	// role-classification AI and all downstream eager enrichment/embedding work.
	excluded := []string{
		" intern ", " internship ", " co-op ", " coop ", " apprentice ", " apprenticeship ", " student ",
		" engineering manager ", " software engineering manager ", " development manager ", " technical manager ",
		" manager ", " director ", " vice president ", " vp ", " head of ",
		" quality assurance ", " qa ", " tester ", " test engineer ", " test automation engineer ",
		" automation test engineer ", " performance test engineer ", " sdet ",
		" analyst ", " product manager ", " program manager ", " project manager ", " scrum master ", " product owner ",
		" support engineer ", " solutions engineer ", " sales engineer ", " recruiter ",
		" help desk ", " desktop support ", " technical support ", " it support ", " customer support ",
		" database administrator ", " db administrator ", " system administrator ", " systems administrator ",
		" network engineer ", " network administrator ", " network operations ",
		" product designer ", " ux designer ", " user experience designer ", " visual designer ", " graphic designer ",
		" data scientist ", " research scientist ",
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
	// More specific compound families (for example DATA_ENGINEERING's
	// "data platform engineer") must come before generic platform roles.
	families := []struct {
		terms []string
		name  string
	}{
		{[]string{" java full stack ", " java full-stack ", " full stack ", " full-stack ", " fullstack ", " mern ", " mean stack "}, "FULLSTACK"},
		{[]string{" backend ", " back end ", " java backend ", " spring boot ", " golang ", " go developer ", " go engineer ", " node.js ", " nodejs ", " api engineer ", " api developer ", " microservices engineer ", " microservice engineer ", " microservices developer ", " microservice developer "}, "BACKEND"},
		{[]string{" frontend ", " front end ", " front-end ", " react developer ", " react engineer ", " angular developer ", " angular engineer ", " vue developer ", " vue engineer ", " ui developer "}, "FRONTEND"},
		{[]string{" data engineer ", " data platform engineer ", " analytics engineer "}, "DATA_ENGINEERING"},
		{[]string{" platform engineer ", " platform developer ", " kubernetes engineer ", " container platform "}, "PLATFORM"},
		{[]string{" infrastructure engineer ", " infrastructure developer ", " database engineer "}, "INFRASTRUCTURE"},
		{[]string{" site reliability ", " sre "}, "SRE"},
		{[]string{" devops ", " devsecops ", " build engineer ", " release engineer "}, "DEVOPS"},
		{[]string{" cloud engineer ", " cloud developer ", " cloud infrastructure "}, "CLOUD"},
		{[]string{" machine learning ", " ml engineer ", " mlops "}, "ML_ENGINEERING"},
		{[]string{" ai engineer ", " artificial intelligence ", " generative ai engineer ", " genai engineer "}, "AI_ENGINEERING"},
		{[]string{" security engineer ", " application security ", " product security engineer "}, "SECURITY_ENGINEERING"},
		{[]string{" ios ", " android ", " mobile "}, "MOBILE"},
		{[]string{" embedded ", " firmware "}, "EMBEDDED"},
		{[]string{" systems software ", " systems engineer software ", " distributed systems engineer ", " distributed systems developer "}, "SYSTEMS"},
		{[]string{
			" software engineering consultant ", " software development consultant ", " software consultant ",
			" application development consultant ", " java consultant ",
			" cloud engineering consultant ", " devops consultant ",
			" data engineering consultant ", " integration consultant ",
		}, "CONSULTING_ENGINEERING"},
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
		" application engineer ", " application developer ", " application software engineer ", " web developer ",
		" java developer ", " java engineer ", " python developer ", " python engineer ",
		" c# developer ", " c# engineer ", " .net developer ", " .net engineer ",
		" javascript developer ", " typescript developer ", " ruby developer ", " ruby engineer ",
		" scala developer ", " scala engineer ", " kotlin developer ", " kotlin engineer ",
		" salesforce developer ", " servicenow developer ", " integration developer ",
		" integration engineer ", " middleware developer ", " enterprise application developer ",
		" enterprise software engineer ", " enterprise software developer ", " technology engineer ",
		" technology developer ",
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
