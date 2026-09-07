package matching

import "strings"

func immigrationRequired(in Input) bool {
	return in.RequiresH1BTransfer ||
		in.RequiresNewH1BCapSponsorship ||
		in.RequiresFutureEmploymentSponsorship ||
		in.GreenCardSupportPreferred ||
		in.GreenCardSupportRequired ||
		in.PermSupportPreferred
}

// AssessImmigration evaluates only role-level posting language. Company
// history/DOL evidence is intentionally a separate later layer. Explicit
// negative role language wins; silence remains UNKNOWN rather than being
// guessed from the employer's reputation.
func AssessImmigration(in Input) ImmigrationAssessment {
	text := strings.ToLower(strings.Join([]string{
		in.WorkAuthorizationRequirements,
		in.JobDescription,
	}, "\n"))

	negative := []string{
		"will not sponsor",
		"do not sponsor",
		"does not sponsor",
		"cannot sponsor",
		"can't sponsor",
		"unable to sponsor",
		"not able to sponsor",
		"no sponsorship",
		"without sponsorship now or in the future",
		"without visa sponsorship",
		"not provide visa sponsorship",
		"not provide sponsorship",
		"not eligible for visa sponsorship",
		"no visa sponsorship available",
		"does not offer sponsorship",
		"must not require sponsorship",
		"cannot provide sponsorship",
	}
	for _, phrase := range negative {
		if strings.Contains(text, phrase) {
			return ImmigrationAssessment{
				Status:     "NOT_SUPPORTED",
				Confidence: "HIGH",
				Evidence:   phrase,
			}
		}
	}

	positive := []string{
		"h-1b sponsorship available",
		"h1b sponsorship available",
		"visa sponsorship available",
		"we sponsor h-1b",
		"we sponsor h1b",
		"h-1b transfer",
		"h1b transfer",
		"support h-1b",
		"support h1b",
		"provide visa sponsorship",
		"provides visa sponsorship",
	}
	for _, phrase := range positive {
		if strings.Contains(text, phrase) {
			return ImmigrationAssessment{
				Status:     "SUPPORTED",
				Confidence: "HIGH",
				Evidence:   phrase,
			}
		}
	}

	return ImmigrationAssessment{Status: "UNKNOWN", Confidence: "LOW"}
}
