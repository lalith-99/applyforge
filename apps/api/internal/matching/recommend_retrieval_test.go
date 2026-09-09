package matching

import (
	"slices"
	"testing"

	"github.com/lalithlochan/applyforge/apps/api/internal/candidateprofile"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobs"
)

func TestRecommendationLexicalTerms_PreservesTargetsAndHighSignalStacks(t *testing.T) {
	terms := recommendationLexicalTerms(candidateprofile.Profile{
		TargetRoles:     []string{"Java Full Stack Developer", "Backend Engineer"},
		CoreSkills:      []string{"Java", "Spring Boot", "Go", "Kafka"},
		SecondarySkills: []string{"Kubernetes", "AWS"},
	})

	for _, expected := range []string{
		"Java Full Stack Developer", "Backend Engineer", "Java",
		"Spring Boot", "Golang", "Kubernetes",
	} {
		if !slices.Contains(terms, expected) {
			t.Fatalf("expected %q in lexical terms: %#v", expected, terms)
		}
	}
	if slices.Contains(terms, "Kafka") || slices.Contains(terms, "AWS") {
		t.Fatalf("generic infrastructure/skill keywords should stay semantic-only: %#v", terms)
	}
}

func TestRecommendationSeniorityEligible_RejectsStaffAndPrincipalForSeniorCandidate(t *testing.T) {
	seniority := "Senior"
	years := int32(6)
	candidate := candidateprofile.Profile{
		Seniority:       &seniority,
		YearsExperience: &years,
		TargetRoles:     []string{"Java Software Engineer", "Backend Engineer"},
	}

	for _, title := range []string{
		"Staff Site Reliability Engineer - Kubernetes",
		"Principal Software Engineer - Compute",
	} {
		if recommendationSeniorityEligible(title, candidate) {
			t.Fatalf("expected %q to be excluded for senior candidate", title)
		}
	}
	if !recommendationSeniorityEligible("Senior Java Backend Engineer", candidate) {
		t.Fatal("expected senior backend role to remain eligible")
	}
}

func TestRecommendationSeniorityEligible_AllowsExplicitStaffTarget(t *testing.T) {
	seniority := "Senior"
	candidate := candidateprofile.Profile{
		Seniority:   &seniority,
		TargetRoles: []string{"Staff Backend Engineer"},
	}
	if !recommendationSeniorityEligible("Staff Backend Engineer", candidate) {
		t.Fatal("explicit staff target should allow staff jobs")
	}
}

func TestRecommendationPreAIRank_PrefersTargetRoleDirection(t *testing.T) {
	candidate := candidateprofile.Profile{
		TargetRoles: []string{"Java Backend Engineer"},
	}
	relevant := RankedJob{
		Job:    jobs.Job{Title: "Senior Java Backend Engineer"},
		Result: Result{TotalScore: 75},
	}
	offTarget := RankedJob{
		Job:    jobs.Job{Title: "Site Reliability Engineer - Kubernetes"},
		Result: Result{TotalScore: 82},
	}
	if recommendationPreAIRank(relevant, candidate) <= recommendationPreAIRank(offTarget, candidate) {
		t.Fatalf("target-role alignment should let relevant Java backend role outrank off-target SRE despite lower raw score")
	}
}
