package jobs

import "testing"

func TestSuccessFactorsBootstrapCandidateDetection(t *testing.T) {
	payload := InspectCompanySourcePayload{
		SourceType: "CUSTOM",
		BoardToken: "SUCCESSFACTORS|careers",
		SourceURL:  "https://careers.example.com",
	}
	if !isSuccessFactorsInspectionPayload(payload) {
		t.Fatal("expected legacy CUSTOM SuccessFactors candidate to use provider verification")
	}

	payload = InspectCompanySourcePayload{
		SourceType: "SUCCESSFACTORS",
		BoardToken: "https://careers.example.com",
		SourceURL:  "https://careers.example.com",
	}
	if !isSuccessFactorsInspectionPayload(payload) {
		t.Fatal("expected first-class SuccessFactors candidate to use provider verification")
	}

	payload = InspectCompanySourcePayload{
		SourceType: "CUSTOM",
		BoardToken: "EIGHTFOLD|careers",
		SourceURL:  "https://careers.example.com",
	}
	if isSuccessFactorsInspectionPayload(payload) {
		t.Fatal("unrelated CUSTOM candidate must not be treated as SuccessFactors")
	}
}
