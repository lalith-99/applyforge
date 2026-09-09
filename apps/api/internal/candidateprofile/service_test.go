package candidateprofile

import (
	"testing"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
)

func TestDedupeNonEmpty_PreservesPrimaryThenAlternativeTargets(t *testing.T) {
	got := dedupeNonEmpty([]string{
		"Java Backend Engineer",
		"Backend Engineer",
		"java backend engineer",
		"",
		"Full Stack Engineer",
	})
	want := []string{"Java Backend Engineer", "Backend Engineer", "Full Stack Engineer"}
	if len(got) != len(want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %#v, got %#v", want, got)
		}
	}
}

func TestCandidateSourceHashChangesWhenRankingInputsChange(t *testing.T) {
	seniority := "Senior"
	years := 5
	workAuth := "H-1B"
	base := aiclient.CandidateProfileRequest{
		TargetRoles:     []string{"Java Backend Engineer"},
		Seniority:       &seniority,
		YearsExperience: &years,
		MasterSkills:    []string{"Java", "Spring Boot", "Kafka"},
		WorkAuthorization: &workAuth,
	}

	baseHash := candidateSourceHash(base)

	changedTarget := base
	changedTarget.TargetRoles = []string{"Java Full Stack Engineer"}
	if candidateSourceHash(changedTarget) == baseHash {
		t.Fatal("target-role changes must invalidate the candidate profile")
	}

	changedYears := base
	newYears := 6
	changedYears.YearsExperience = &newYears
	if candidateSourceHash(changedYears) == baseHash {
		t.Fatal("years-experience changes must invalidate the candidate profile")
	}

	changedSkills := base
	changedSkills.MasterSkills = append([]string{}, base.MasterSkills...)
	changedSkills.MasterSkills = append(changedSkills.MasterSkills, "Kubernetes")
	if candidateSourceHash(changedSkills) == baseHash {
		t.Fatal("resume-skill changes must invalidate the candidate profile")
	}
}

func TestCandidateSourceHashIncludesExperienceEvidence(t *testing.T) {
	base := aiclient.CandidateProfileRequest{
		TargetRoles: []string{"Backend Engineer"},
		Experiences: []aiclient.CandidateExperience{
			{
				Company:      "Example",
				Title:        "Software Engineer",
				Bullets:      []string{"Built APIs"},
				Technologies: []string{"Java"},
			},
		},
	}
	changed := base
	changed.Experiences = append([]aiclient.CandidateExperience{}, base.Experiences...)
	changed.Experiences[0] = base.Experiences[0]
	changed.Experiences[0].Bullets = []string{"Built Kafka pipelines"}

	if candidateSourceHash(base) == candidateSourceHash(changed) {
		t.Fatal("resume experience changes must invalidate the candidate profile")
	}
}
