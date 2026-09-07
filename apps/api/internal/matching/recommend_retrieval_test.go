package matching

import (
	"slices"
	"testing"

	"github.com/lalithlochan/applyforge/apps/api/internal/candidateprofile"
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
