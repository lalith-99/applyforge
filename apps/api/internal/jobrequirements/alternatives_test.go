package jobrequirements

import (
	"testing"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
)

func TestWithAlternativeRequirements_RobloxPythonOrGoBecomesOneRequirement(t *testing.T) {
	description := "You Have - 7+ years of proven experience as a software engineer. - Expertise in Python or Golang; - Strong understanding of system design."
	reqs := withAlternativeRequirements(Requirements{}, description)

	if len(reqs.RequiredSkills) != 1 {
		t.Fatalf("expected exactly one alternative requirement, got %+v", reqs.RequiredSkills)
	}
	got := reqs.RequiredSkills[0]
	if got.Category != "alternative" || len(got.Alternatives) != 2 {
		t.Fatalf("expected alternative requirement metadata, got %+v", got)
	}
	if !sameAlternativeSkill("Python", got.Alternatives) || !sameAlternativeSkill("Golang", got.Alternatives) {
		t.Fatalf("expected Python/Go alternatives, got %+v", got.Alternatives)
	}
}

func TestWithAlternativeRequirements_RedditOneOrMoreLanguagesBecomesOneRequirement(t *testing.T) {
	description := "Software development experience in one or more general purpose programming languages; Python, Go, Rust, Java, C++."
	reqs := withAlternativeRequirements(Requirements{}, description)

	if len(reqs.RequiredSkills) != 1 {
		t.Fatalf("expected one language-group requirement, got %+v", reqs.RequiredSkills)
	}
	if len(reqs.RequiredSkills[0].Alternatives) != 5 {
		t.Fatalf("expected five alternatives inside one requirement, got %+v", reqs.RequiredSkills[0])
	}
}

func TestWithAlternativeRequirements_DoesNotRemoveIndependentSameSkillRequirement(t *testing.T) {
	description := "Expertise in Python or Golang. Python is required for our data tooling."
	reqs := Requirements{
		RequiredSkills: []aiclient.SkillRequirement{
			{
				NormalizedName: "Python",
				OriginalText:   "Python is required for our data tooling",
				Importance:     "required",
			},
		},
	}

	got := withAlternativeRequirements(reqs, description)
	if len(got.RequiredSkills) != 2 {
		t.Fatalf("expected independent Python requirement plus alternative group, got %+v", got.RequiredSkills)
	}
}
