package jobrequirements

import (
	"testing"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
)

func TestSanitizeRequirementTypesMovesDegreeOutOfSkills(t *testing.T) {
	required := []aiclient.SkillRequirement{
		{
			NormalizedName: "linux",
			OriginalText:   "Strong Linux OS experience",
			Importance:     "required",
		},
		{
			NormalizedName: "bachelor's degree in computer science or engineering",
			OriginalText:   "Bachelor's in CS, Engineering, or related field",
			Importance:     "required",
		},
	}
	preferred := []aiclient.SkillRequirement{}
	education := []string{}
	keywords := []string{"linux", "bachelor's degree in computer science or engineering"}

	required, preferred, education, keywords = sanitizeRequirementTypes(
		required,
		preferred,
		education,
		keywords,
	)

	if len(required) != 1 || required[0].NormalizedName != "linux" {
		t.Fatalf("expected only linux as required skill, got %+v", required)
	}
	if len(preferred) != 0 {
		t.Fatalf("unexpected preferred skills: %+v", preferred)
	}
	if len(education) != 1 ||
		education[0] != "Bachelor's in CS, Engineering, or related field" {
		t.Fatalf("expected degree moved to education requirements, got %+v", education)
	}
	if len(keywords) != 1 || keywords[0] != "linux" {
		t.Fatalf("expected degree removed from technical keywords, got %+v", keywords)
	}
}
