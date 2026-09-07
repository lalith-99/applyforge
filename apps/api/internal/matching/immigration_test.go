package matching

import "testing"

func TestAssessImmigration_ExplicitNoSponsorshipIsHardFailure(t *testing.T) {
	in := Input{
		RequiresH1BTransfer: true,
		JobDescription: "Applicants must be authorized to work in the United States without sponsorship now or in the future.",
	}
	result := CheckEligibility(in)
	if result.Eligible {
		t.Fatalf("expected explicit no-sponsorship language to make job ineligible: %+v", result)
	}
	if result.Immigration.Status != "NOT_SUPPORTED" || result.Immigration.Confidence != "HIGH" {
		t.Fatalf("unexpected immigration assessment: %+v", result.Immigration)
	}
}

func TestAssessImmigration_SilenceIsWarningNotRejection(t *testing.T) {
	in := Input{
		RequiresH1BTransfer: true,
		JobDescription: "Build distributed systems in Go and Kubernetes.",
	}
	result := CheckEligibility(in)
	if !result.Eligible {
		t.Fatalf("silent posting must not be automatically rejected: %+v", result)
	}
	if result.Immigration.Status != "UNKNOWN" || len(result.Warnings) == 0 {
		t.Fatalf("expected UNKNOWN sponsorship warning: %+v", result)
	}
}

func TestAssessImmigration_ExplicitH1BTransferSupport(t *testing.T) {
	in := Input{
		RequiresH1BTransfer: true,
		JobDescription: "We support H-1B transfer for qualified candidates.",
	}
	result := CheckEligibility(in)
	if !result.Eligible || result.Immigration.Status != "SUPPORTED" {
		t.Fatalf("expected supported transfer role: %+v", result)
	}
}

func TestAssessImmigration_NegativeLanguageOverridesPositiveHistoryLikeLanguage(t *testing.T) {
	in := Input{
		RequiresH1BTransfer: true,
		JobDescription: "We have sponsored H-1B workers historically. This role will not sponsor employment visas.",
	}
	result := CheckEligibility(in)
	if result.Eligible || result.Immigration.Status != "NOT_SUPPORTED" {
		t.Fatalf("role-level negative language must win: %+v", result)
	}
}


func TestAssessImmigration_HistoricalH1BEvidenceIsSecondarySignal(t *testing.T) {
	in := Input{
		RequiresH1BTransfer:        true,
		JobDescription:             "Build distributed systems in Go.",
		CompanyH1BCertifiedCases:   14,
		CompanyH1BTotalCases:       16,
		CompanyEvidenceLatestFY:    2026,
		CompanyEvidenceEmployers:   []string{"Acme Technologies, Inc."},
	}
	result := CheckEligibility(in)
	if !result.Eligible {
		t.Fatalf("historical employer evidence must not make a role ineligible: %+v", result)
	}
	if result.Immigration.Status != "HISTORICAL_SUPPORT" || result.Immigration.EvidenceSource != "DOL_HISTORY" {
		t.Fatalf("expected historical support assessment: %+v", result.Immigration)
	}
	if result.Immigration.H1BCertifiedCases != 14 || result.Immigration.LatestEvidenceFiscalYear != 2026 {
		t.Fatalf("expected DOL evidence metadata: %+v", result.Immigration)
	}
}

func TestAssessImmigration_ExplicitNoSponsorshipOverridesDOLHistory(t *testing.T) {
	in := Input{
		RequiresH1BTransfer:      true,
		JobDescription:           "This role will not sponsor employment visas.",
		CompanyH1BCertifiedCases: 250,
		CompanyPERMCertifiedCases: 40,
		CompanyEvidenceLatestFY:  2026,
	}
	result := CheckEligibility(in)
	if result.Eligible || result.Immigration.Status != "NOT_SUPPORTED" {
		t.Fatalf("explicit role restriction must override employer history: %+v", result)
	}
	if result.Immigration.EvidenceSource != "JOB_POSTING" {
		t.Fatalf("expected role-level evidence source: %+v", result.Immigration)
	}
}

func TestAssessImmigration_PERMHistoryOnlyCountsWhenRelevant(t *testing.T) {
	h1bOnly := Input{
		RequiresH1BTransfer:       true,
		CompanyPERMCertifiedCases: 20,
		CompanyEvidenceLatestFY:   2026,
	}
	if got := AssessImmigration(h1bOnly); got.Status != "UNKNOWN" {
		t.Fatalf("PERM history alone must not imply H-1B transfer support: %+v", got)
	}

	permPreferred := Input{
		PermSupportPreferred:      true,
		CompanyPERMCertifiedCases: 20,
		CompanyEvidenceLatestFY:   2026,
	}
	if got := AssessImmigration(permPreferred); got.Status != "HISTORICAL_SUPPORT" {
		t.Fatalf("PERM history should be relevant to PERM preference: %+v", got)
	}
}
