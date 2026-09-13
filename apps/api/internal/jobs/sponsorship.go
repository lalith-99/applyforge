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
}

var explicitSponsorshipSupportPhrases = []string{
	"h-1b sponsorship available",
	"h1b sponsorship available",
	"visa sponsorship available",
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
	"h-1b portability",
	"h1b portability",
	"support h-1b",
	"support h1b",
	"h-1b sponsorship support",
	"h1b sponsorship support",
	"provide visa sponsorship",
	"provides visa sponsorship",
}

func explicitSponsorshipDenied(text string) bool {
	value := strings.ToLower(text)
	for _, phrase := range explicitSponsorshipDenialPhrases {
		if strings.Contains(value, phrase) {
			return true
		}
	}
	return false
}

func explicitSponsorshipSupported(text string) bool {
	value := strings.ToLower(text)
	for _, phrase := range explicitSponsorshipSupportPhrases {
		if strings.Contains(value, phrase) {
			return true
		}
	}
	return false
}
