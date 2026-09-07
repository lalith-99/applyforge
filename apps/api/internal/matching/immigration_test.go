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
