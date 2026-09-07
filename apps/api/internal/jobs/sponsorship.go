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

func explicitSponsorshipDenied(text string) bool {
	value := strings.ToLower(text)
	for _, phrase := range explicitSponsorshipDenialPhrases {
		if strings.Contains(value, phrase) {
			return true
		}
	}
	return false
}
