package jobs

import "strings"

var explicitSponsorshipDenialPhrases = []string{
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
	"no immigration sponsorship",
	"no employment visa sponsorship",
	"sponsorship is not available",
	"visa sponsorship is not available",
	"h-1b sponsorship is not available",
	"h1b sponsorship is not available",
}

var explicitSponsorshipSupportPhrases = []string{
	"h-1b sponsorship available",
	"h1b sponsorship available",
	"visa sponsorship available",
	"employment visa sponsorship available",
	"visa sponsorship provided",
	"sponsorship is available",
	"sponsorship available",
	"we sponsor h-1b",
	"we sponsor h1b",
	"sponsor h-1b",
	"sponsor h1b",
	"h-1b visa sponsorship",
	"h1b visa sponsorship",
	"h-1b transfer",
	"h1b transfer",
	"h-1b transfers",
	"h1b transfers",
	"transfer an existing h-1b",
	"transfer an existing h1b",
	"transfer existing h-1b",
	"transfer existing h1b",
	"h-1b portability",
	"h1b portability",
	"support h-1b",
	"support h1b",
	"supports h-1b",
	"supports h1b",
	"h-1b sponsorship support",
	"h1b sponsorship support",
	"provide visa sponsorship",
	"provides visa sponsorship",
	"provide employment visa sponsorship",
	"provides employment visa sponsorship",
}

// normalizeSponsorshipText makes provider/ATS typography irrelevant to the
// cheap prefilter. In particular, many career pages use Unicode non-breaking
// hyphens/dashes in "H-1B"; without normalization those explicit positives can
// be missed and an otherwise eligible job can be filtered out before ranking.
func normalizeSponsorshipText(text string) string {
	value := strings.ToLower(text)
	value = strings.NewReplacer(
		"‐", "-", // hyphen
		"‑", "-", // non-breaking hyphen
		"‒", "-", // figure dash
		"–", "-", // en dash
		"—", "-", // em dash
		"−", "-", // minus sign
		"\u00a0", " ", // non-breaking space
	).Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func explicitSponsorshipDenied(text string) bool {
	value := normalizeSponsorshipText(text)
	for _, phrase := range explicitSponsorshipDenialPhrases {
		if strings.Contains(value, phrase) {
			return true
		}
	}
	return false
}

func explicitSponsorshipSupported(text string) bool {
	value := normalizeSponsorshipText(text)
	// Explicit role-level denial always wins. Keeping the precedence here makes
	// the prefilter safe even when callers forget to perform the denial check
	// first, and mirrors matching.AssessImmigration's evidence ordering.
	if explicitSponsorshipDenied(value) {
		return false
	}
	for _, phrase := range explicitSponsorshipSupportPhrases {
		if strings.Contains(value, phrase) {
			return true
		}
	}
	return false
}
