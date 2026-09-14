package jobs

import "testing"

func TestExplicitSponsorshipDenied(t *testing.T) {
	negative := []string{
		"We will not sponsor applicants for this role.",
		"Candidates must be able to work without sponsorship now or in the future.",
		"No visa sponsorship available.",
		"Employment visa sponsorship is not available for this position.",
		"No immigration sponsorship will be provided.",
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

func TestExplicitSponsorshipSupported(t *testing.T) {
	positive := []string{
		"H-1B transfer support is available for qualified candidates.",
		"We provide visa sponsorship.",
		"The company supports H1B portability.",
		"We can transfer an existing H-1B for this position.",
		"Employment visa sponsorship available for qualified candidates.",
		"The company supports H‑1B portability.",
	}
	for _, text := range positive {
		if !explicitSponsorshipSupported(text) {
			t.Fatalf("expected explicit support for %q", text)
		}
	}

	for _, text := range []string{
		"Must be authorized to work in the United States.",
		"Sponsorship requirements will be discussed.",
	} {
		if explicitSponsorshipSupported(text) {
			t.Fatalf("ambiguous text must not be treated as explicit support: %q", text)
		}
	}
}

func TestExplicitSponsorshipDenialOverridesPositivePhrase(t *testing.T) {
	text := "H-1B sponsorship is not available for this role, although other positions may provide visa sponsorship."
	if !explicitSponsorshipDenied(text) {
		t.Fatalf("expected explicit denial for mixed evidence text")
	}
	if explicitSponsorshipSupported(text) {
		t.Fatalf("explicit role-level denial must override positive sponsorship language")
	}
}

func TestNormalizeSponsorshipTextHandlesATSTypography(t *testing.T) {
	got := normalizeSponsorshipText("  H‑1B\u00a0transfer   support  ")
	if got != "h-1b transfer support" {
		t.Fatalf("unexpected normalized sponsorship text: %q", got)
	}
}
