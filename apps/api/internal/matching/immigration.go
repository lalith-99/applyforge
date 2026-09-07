package matching

import (
	"fmt"
	"strings"
)

func immigrationRequired(in Input) bool {
	return h1bSupportRequired(in) ||
		in.GreenCardSupportPreferred ||
		in.GreenCardSupportRequired ||
		in.PermSupportPreferred
}

func h1bSupportRequired(in Input) bool {
	return in.RequiresH1BTransfer ||
		in.RequiresNewH1BCapSponsorship ||
		in.RequiresFutureEmploymentSponsorship ||
		looksLikeH1B(in.ImmigrationStatus) ||
		looksLikeH1B(in.WorkAuthorization)
}

func looksLikeH1B(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.NewReplacer("-", "", " ", "", "_", "").Replace(normalized)
	return strings.Contains(normalized, "h1b")
}

func permSupportRelevant(in Input) bool {
	return in.GreenCardSupportPreferred ||
		in.GreenCardSupportRequired ||
		in.PermSupportPreferred
}

// AssessImmigration applies evidence in strict precedence order:
//  1. explicit negative role text
//  2. explicit positive role text
//  3. recent historical DOL employer evidence
//  4. unknown
//
// Historical employer evidence is deliberately not called "SUPPORTED": it
// proves that the employer has recently used H-1B LCA/PERM pathways, not that
// this particular role will sponsor.
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
				Status:                   "NOT_SUPPORTED",
				Confidence:               "HIGH",
				Evidence:                 phrase,
				EvidenceSource:           "JOB_POSTING",
				H1BCertifiedCases:        in.CompanyH1BCertifiedCases,
				PERMCertifiedCases:       in.CompanyPERMCertifiedCases,
				LatestEvidenceFiscalYear: in.CompanyEvidenceLatestFY,
				MatchedEmployers:         in.CompanyEvidenceEmployers,
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
				Status:                   "SUPPORTED",
				Confidence:               "HIGH",
				Evidence:                 phrase,
				EvidenceSource:           "JOB_POSTING",
				H1BCertifiedCases:        in.CompanyH1BCertifiedCases,
				PERMCertifiedCases:       in.CompanyPERMCertifiedCases,
				LatestEvidenceFiscalYear: in.CompanyEvidenceLatestFY,
				MatchedEmployers:         in.CompanyEvidenceEmployers,
			}
		}
	}

	relevantH1B := h1bSupportRequired(in) && in.CompanyH1BCertifiedCases > 0
	relevantPERM := permSupportRelevant(in) && in.CompanyPERMCertifiedCases > 0
	if relevantH1B || relevantPERM {
		var parts []string
		if relevantH1B {
			parts = append(parts, fmt.Sprintf("%d certified H-1B LCA case signal(s)", in.CompanyH1BCertifiedCases))
		}
		if relevantPERM {
			parts = append(parts, fmt.Sprintf("%d certified PERM case signal(s)", in.CompanyPERMCertifiedCases))
		}
		if in.CompanyEvidenceLatestFY > 0 {
			parts = append(parts, fmt.Sprintf("latest FY%d", in.CompanyEvidenceLatestFY))
		}

		confidence := "LOW"
		relevantCount := 0
		if relevantH1B {
			relevantCount += in.CompanyH1BCertifiedCases
		}
		if relevantPERM {
			relevantCount += in.CompanyPERMCertifiedCases
		}
		if relevantCount >= 3 {
			confidence = "MEDIUM"
		}

		return ImmigrationAssessment{
			Status:                   "HISTORICAL_SUPPORT",
			Confidence:               confidence,
			Evidence:                 strings.Join(parts, "; "),
			EvidenceSource:           "DOL_HISTORY",
			H1BCertifiedCases:        in.CompanyH1BCertifiedCases,
			PERMCertifiedCases:       in.CompanyPERMCertifiedCases,
			LatestEvidenceFiscalYear: in.CompanyEvidenceLatestFY,
			MatchedEmployers:         in.CompanyEvidenceEmployers,
		}
	}

	return ImmigrationAssessment{
		Status:                   "UNKNOWN",
		Confidence:               "LOW",
		EvidenceSource:           "NONE",
		H1BCertifiedCases:        in.CompanyH1BCertifiedCases,
		PERMCertifiedCases:       in.CompanyPERMCertifiedCases,
		LatestEvidenceFiscalYear: in.CompanyEvidenceLatestFY,
		MatchedEmployers:         in.CompanyEvidenceEmployers,
	}
}
