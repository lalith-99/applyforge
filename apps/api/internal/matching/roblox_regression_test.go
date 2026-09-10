package matching

import (
	"strings"
	"testing"
	"time"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
)

func TestToSkillRequirements_PreservesAlternativeGroup(t *testing.T) {
	got := toSkillRequirements([]aiclient.SkillRequirement{
		{
			NormalizedName: "Go or Python",
			Importance:     "required",
			Alternatives:   []string{"Go", "Python"},
		},
	})

	if len(got) != 1 {
		t.Fatalf("expected one requirement group, got %d", len(got))
	}
	if len(got[0].Alternatives) != 2 || got[0].Alternatives[0] != "go" || got[0].Alternatives[1] != "python" {
		t.Fatalf("expected normalized alternatives to survive conversion, got %+v", got[0])
	}
}

func TestScore_AlternativeLanguageRequirementCountsAsOneAndPythonSatisfiesIt(t *testing.T) {
	input := Input{
		RequiredSkills: []SkillRequirement{
			{
				NormalizedName: "go or python",
				Importance:     "required",
				Alternatives:   []string{"go", "python"},
			},
		},
		CandidateSkills:    map[string]bool{"python": true},
		CandidateSeniority: "senior",
		JobSeniority:       "senior",
		Responsibilities:   []string{"Apply expertise in Python or Golang to build large-scale systems."},
		FirstSeenAt:        time.Now(),
	}

	result := Score(input)
	if len(result.MissingRequiredSkills) != 0 {
		t.Fatalf("Python should satisfy the Python-or-Go requirement, missing=%v", result.MissingRequiredSkills)
	}
	if result.CurrentProfileMatch != 100 {
		t.Fatalf("expected current profile match 100 for the single satisfied OR requirement, got %d", result.CurrentProfileMatch)
	}
	if !strings.Contains(result.Explanation, "1/1 required skills") {
		t.Fatalf("expected explanation to report 1/1 rather than 0/0 or 2/2, got %q", result.Explanation)
	}
}

func TestScore_AlternativeLanguageRequirementMissingCountsOnce(t *testing.T) {
	input := Input{
		RequiredSkills: []SkillRequirement{
			{
				NormalizedName: "go or python",
				Importance:     "required",
				Alternatives:   []string{"go", "python"},
			},
		},
		CandidateSkills: map[string]bool{"java": true},
		FirstSeenAt:     time.Now(),
	}

	result := Score(input)
	if len(result.MissingRequiredSkills) != 1 || result.MissingRequiredSkills[0] != "go or python" {
		t.Fatalf("expected exactly one missing alternative requirement, got %v", result.MissingRequiredSkills)
	}
	if result.CurrentProfileMatch != 0 {
		t.Fatalf("expected 0 current profile match when neither alternative is present, got %d", result.CurrentProfileMatch)
	}
}

func TestScore_TargetProfileCanSatisfyMissingAlternativeWithOneTargetSkill(t *testing.T) {
	input := Input{
		RequiredSkills: []SkillRequirement{
			{
				NormalizedName: "go or python",
				Importance:     "required",
				Alternatives:   []string{"go", "python"},
			},
		},
		CandidateSkills:       map[string]bool{"java": true},
		CandidateTargetSkills: map[string]bool{"go": true},
		FirstSeenAt:           time.Now(),
	}

	result := Score(input)
	if result.CurrentProfileMatch != 0 || result.TargetProfileMatch != 100 {
		t.Fatalf("expected target Go to satisfy the OR group only in target profile, current=%d target=%d", result.CurrentProfileMatch, result.TargetProfileMatch)
	}
}

func TestAssessImmigration_RobloxConditionalH1BRestrictionOverridesDOLHistory(t *testing.T) {
	input := Input{
		RequiresH1BTransfer:       true,
		CompanyH1BCertifiedCases:  190,
		CompanyPERMCertifiedCases: 53,
		CompanyEvidenceLatestFY:   2026,
		JobDescription: "For US based roles only, the Company may not be able to employ candidates for this role " +
			"who have United States work authorization related to certain U.S. visa categories, or support future H-1B sponsorship at this time.",
	}

	result := CheckEligibility(input)
	if !result.Eligible {
		t.Fatalf("conditional wording should remain a caution rather than an automatic hard rejection: %+v", result)
	}
	if result.Immigration.Status != "UNKNOWN" || result.Immigration.Confidence != "MEDIUM" {
		t.Fatalf("expected medium-confidence current-role caution, got %+v", result.Immigration)
	}
	if result.Immigration.EvidenceSource != "JOB_POSTING" {
		t.Fatalf("current posting must outrank historical DOL evidence, got %+v", result.Immigration)
	}
	if len(result.Warnings) == 0 {
		t.Fatalf("expected an H-1B caution warning for the conditional posting")
	}
}
