package matching

import (
	"strings"
	"testing"
)

func TestAssessImmigration_RobloxConditionalRestrictionOverridesDOLHistory(t *testing.T) {
	in := Input{
		RequiresH1BTransfer:       true,
		JobDescription:            "For US based roles only, please note the Company may not be able to employ candidates for this role who have United States work authorization related to certain U.S. visa categories, or support future H-1B sponsorship at this time.",
		CompanyH1BCertifiedCases:  190,
		CompanyPERMCertifiedCases: 53,
		CompanyEvidenceLatestFY:   2026,
	}

	result := CheckEligibility(in)
	if !result.EligibilityCompatibleForTest() {
		// The wording is conditional ("may not"), so this is a warning rather
		// than an automatic rejection until the recruiter confirms the policy.
		t.Fatalf("conditional H-1B restriction should remain reviewable: %+v", result)
	}
	if result.Immigration.Status != "UNKNOWN" || result.Immigration.EvidenceSource != "JOB_POSTING" {
		t.Fatalf("current role-level caveat must outrank DOL history: %+v", result.Immigration)
	}
	if result.Immigration.Confidence != "MEDIUM" {
		t.Fatalf("expected medium-confidence conditional warning: %+v", result.Immigration)
	}
	if len(result.Warnings) == 0 || !strings.Contains(result.Warnings[len(result.Warnings)-1], "H-1B CAUTION") {
		t.Fatalf("expected prominent H-1B caution, got %+v", result.Warnings)
	}
}

// Small test helper keeps the assertion above readable without exposing new
// production API surface.
func (r EligibilityResult) EligibilityCompatibleForTest() bool {
	return r.Eligible
}
