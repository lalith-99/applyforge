package jobs

import "testing"

func TestExplicitSponsorshipDenied(t *testing.T) {
	negative := []string{
		"We will not sponsor applicants for this role.",
		"Candidates must be able to work without sponsorship now or in the future.",
		"No visa sponsorship available.",
	}
	for _, text := range negative {
		if !explicitSponsorshipDenied(text) {
			t.Fatalf("expected explicit denial for %q", text)
		}
	}

	ambiguous := []string{
		"Must be authorized to work in the United States.",
		"Please indicate whether you may need sponsorship in the future.",
		"H-1B transfer support may be considered case by case.",
	}
	for _, text := range ambiguous {
		if explicitSponsorshipDenied(text) {
			t.Fatalf("ambiguous/positive text must not be treated as explicit denial: %q", text)
		}
	}
}
